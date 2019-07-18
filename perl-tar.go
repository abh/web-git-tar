package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-billy.v4/osfs"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
	// "gopkg.in/src-d/go-git.v4/storage/filesystem/dotgit"
)

type GitTar struct {
	RepoURL   string
	Directory string
}

func (gt *GitTar) Setup() error {
	if _, err := os.Stat(gt.Directory); err == nil {
		log.Printf("Opening existing clone")
		_, err := git.PlainOpen(gt.Directory)
		if err != nil {
			return fmt.Errorf("could not open %q: %s", gt.Directory, err)
		}
	} else {
		_, err := git.PlainClone(gt.Directory, true, &git.CloneOptions{
			URL:               gt.RepoURL,
			Progress:          os.Stdout,
			NoCheckout:        true,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		})
		if err != nil {
			return fmt.Errorf("could not clone %q to %q: %s", gt.RepoURL, gt.Directory, err)
		}
	}

	log.Printf("has git repo!")
	return nil
}

func (gt *GitTar) Update() error {
	r, err := gt.Load()
	if err != nil {
		return err
	}
	remotes, err := r.Remotes()
	if err != nil {
		return fmt.Errorf("could not list remotes: %s", err)
	}
	for _, remote := range remotes {
		remote.Config().URLs[0] = gt.RepoURL
		log.Printf("Updating %q from %q", remote.Config().Name, remote.Config().URLs[0])
		remote.Fetch(&git.FetchOptions{
			Progress: os.Stdout,
			Force:    true,
		})
		// r.Fetch(&git.FetchOptions{
		// 	RemoteName: remote.Name(),
		// })
	}
	return nil
}

func (gt *GitTar) Load() (*git.Repository, error) {
	fs := osfs.New(gt.Directory)
	workFS := memfs.New()
	storageCache := cache.NewObjectLRUDefault()
	stor := filesystem.NewStorage(fs, storageCache)
	r, err := git.Open(stor, workFS)
	if err != nil {
		return nil, fmt.Errorf("could not open %q: %s", gt.Directory, err)
	}
	return r, nil
}

func main() {

	gt := GitTar{
		RepoURL:   "git://perl5.git.perl.org/perl.git",
		Directory: "/tmp/perl",
	}
	err := gt.Setup()
	if err != nil {
		log.Fatalf("could not setup git-tar: %q", err)
	}

	r, err := gt.Load()

	headRef, err := r.Head()
	if err != nil {
		log.Fatalf("head err: %s", err)
	}
	commit, err := r.CommitObject(headRef.Hash())
	if err != nil {
		log.Fatalf("could not get commit %s: %s", headRef.Hash(), err)
	}
	fmt.Printf("head commit: %s", commit.String())

	// local branches
	branches, err := r.Branches()
	if err != nil {
		log.Fatalf("could not get branches from %q (%q)", gt.RepoURL, gt.Directory)
	}

	for {
		branch, err := branches.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("branch read error: %s", err)
		}
		log.Printf("branch: %q", branch.Name().String())
		log.Printf("branch data: %+v", branch)
	}

	w, err := r.Worktree()
	if err != nil {
		log.Printf("could not get work tree: %s", err)
	}

	log.Printf("git checkout %s to %s", commit.ID().String(), w.Filesystem.Root())
	err = w.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(commit.ID().String()),
		Force: true,
		Keep:  false,
	})

	commitTime := commit.Committer.When

	xattrs := map[string]string{
		"comment": commit.ID().String(),
	}

	patchLine, err := gt.GetPatchLine(r, commit.ID().String())
	if err != nil {
		log.Fatal(err)
		// return err
	}

	pfh, err := w.Filesystem.Create(".patch")
	if err != nil {
		log.Fatal(err)
		// return err
	}
	_, err = pfh.Write([]byte(patchLine))
	if err != nil {
		log.Fatal(err)
		// return err
	}
	pfh.Close()

	err = makeTar(w.Filesystem, "/tmp/perl.tar", commitTime, xattrs)
	if err != nil {
		log.Fatalf("error making tar: %s", err)
	}
}

func makeTar(fs billy.Filesystem, file string, fileTime time.Time, attrs map[string]string) error {
	log.Printf("writing to %q", file)
	fh, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("open file %q: %s", file, err)
	}
	defer fh.Close()
	tarWriter := tar.NewWriter(fh)

	if attrs != nil {
		header := &tar.Header{
			Typeflag:   tar.TypeXGlobalHeader,
			ModTime:    fileTime,
			PAXRecords: attrs,
		}
		tarWriter.WriteHeader(header)
	}

	return memFsTarFile(tarWriter, fileTime, "/", "perl-123", "/", fs)
}

func memFsTarFile(tarWriter *tar.Writer, fileTime time.Time, source string, baseDir string, path string, fs billy.Filesystem) error {

	log.Printf("running memFsTarFile for %q, %q, %q", source, baseDir, path)
	info, err := fs.Stat(path)
	if err != nil {
		return err
	}

	log.Printf("info for %q: %+v", path, info)

	header, err := tar.FileInfoHeader(info, path)
	if err != nil {
		return err
	}
	if header.Uid == 0 {
		header.Uname = "root"
	}
	if header.Gid == 0 {
		header.Gname = "root"
	}
	if !fileTime.IsZero() {
		header.ModTime = fileTime
		// header.ChangeTime = fileTime
	}

	if baseDir != "" {
		header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
	}

	header.Mode = header.Mode | 0664

	if info.IsDir() {
		header.Name += "/"
		header.Mode = header.Mode | 0111
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return err
	}

	if info.IsDir() {
		log.Printf("%q is a DIR!", path)
		dirEntries, err := fs.ReadDir(path)
		if err != nil {
			return err
		}

		var dirPaths []string
		for _, i := range dirEntries {
			if i.IsDir() {
				dirPaths = append(dirPaths, i.Name()+"/")
			} else {
				dirPaths = append(dirPaths, i.Name())

			}
		}
		sort.Strings(dirPaths)

		for _, p := range dirPaths {
			newPath := filepath.Join(path, p)
			log.Printf("saw file %q, calling memFSTarFile(%q, %q, %q", p, source, baseDir, newPath)
			if err = memFsTarFile(tarWriter, fileTime, source, baseDir, newPath, fs); err != nil {
				return err
			}
		}
		return nil
	}

	// not a directory
	file, err := fs.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.CopyN(tarWriter, file, info.Size())
	if err != nil && err != io.EOF {
		return err
	}
	err = tarWriter.Flush()
	if err != nil {
		return err
	}

	return nil
}

func (gt *GitTar) getSHA1(id string) (string, error) {

	var ferr error
	for _, target := range []string{"remotes/origin/" + id, id} {

		bsha1, err := gt.runGit("/tmp/perl", []string{"rev-parse", "--verify", target})
		if err != nil {
			ferr = err
			continue
		}
		return string(bsha1), nil
	}
	return "", ferr
}

func (gt *GitTar) GetPatchLine(r *git.Repository, id string) (string, error) {

	sha1, err := gt.getSHA1(id)
	if err != nil {
		return "", err
	}

	log.Printf("sha1: %q", sha1)

	// look without, and use the result if it is tags/

	branchCandidates := []string{
		"blead",
		"maint-5.10",
		"maint-5.8",
		"maint-5.8-dor",
		"maint-5.6",
		"maint-5.005",
		"maint-5.004",
		// and more generalized searches...
		"refs/heads/*",
		"refs/remotes/*",
		"refs/*",
		"tags/*",
	}

	var branch string
	for _, name := range branchCandidates {
		refs := "remotes/origin/" + name

		bbranch, err := gt.runGit("/tmp/perl", []string{"name-rev", "--name-only", "--refs=" + refs, sha1})
		if err != nil {
			return "", err
		}

		if string(bbranch) != "undefined" {
			branch = string(bbranch)
			break
		}
		// log.Printf("for ref %q got branch %q", refs, branch)
		// last if $branch ne 'undefined';
	}

	strip := []string{"origin/", "refs/heads/", "refs/remotes", "refs/"}
	for _, s := range strip {
		if strings.HasPrefix(branch, s) {
			branch = branch[len(s):]
		}
	}
	suffixDelims := []string{"~", "^"}
	for _, d := range suffixDelims {
		if n := strings.Index(branch, d); n > 0 {
			branch = branch[0:n]
		}
	}

	describeb, err := gt.runGit("/tmp/perl", []string{"describe", sha1})
	if err != nil {
		return "", err
	}

	commit, err := r.CommitObject(plumbing.NewHash(sha1))
	if err != nil {
		return "", fmt.Errorf("loading commit: %s", err)
	}

	pl := fmt.Sprintf("%s %s %s %s",
		branch,
		commit.Committer.When.UTC().Format("2006-01-02.15:04:05"),
		sha1,
		describeb)

	return pl, nil
}

func (gt *GitTar) runGit(dir string, args []string) ([]byte, error) {
	cmdName := "git"
	cmdArgs := args

	err := os.Chdir(dir)
	if err != nil {
		return nil, err
	}

	var cmdOut []byte
	if cmdOut, err = exec.Command(cmdName, cmdArgs...).Output(); err != nil {
		return nil, fmt.Errorf("error running git %s command: %s (%s)", args, err, cmdOut)
	}
	cmdOut = bytes.TrimSpace(cmdOut)
	return cmdOut, nil
	// sha := string(cmdOut)
	// firstSix := sha[:6]
	// fmt.Println("The first six chars of the SHA at HEAD in this repo are", firstSix)
}
