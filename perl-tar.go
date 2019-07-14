package main

import (
	"fmt"
	"io"
	"archive/tar"
	"log"
	"os"

	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-billy.v4/osfs"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
	// "gopkg.in/src-d/go-git.v4/storage/filesystem/dotgit"
)

func main() {

	url := "git://perl5.git.perl.org/perl.git"
	directory := "/tmp/perl"

	if _, err := os.Stat(directory); err == nil {
		log.Printf("Opening existing clone")
		r, err := git.PlainOpen(directory)
		if err != nil {
			log.Fatalf("could not open %q: %s", directory, err)
		}
		remotes, err := r.Remotes()
		if err != nil {
			log.Fatalf("could not list remotes: %s", err)
		}
		for _, remote := range remotes {
			log.Printf("Updating %q from %q", remote.Config().Name, remote.Config().URLs[0])
			remote.Fetch(&git.FetchOptions{
				Progress: os.Stdout,
				Force:    true,
			})
			// r.Fetch(&git.FetchOptions{
			// 	RemoteName: remote.Name(),
			// })
		}
	} else {
		_, err := git.PlainClone(directory, true, &git.CloneOptions{
			URL:               url,
			Progress:          os.Stdout,
			NoCheckout:        true,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		})
		if err != nil {
			log.Fatalf("could not clone %q to %q: %s", url, directory, err)
		}
	}

	log.Printf("has git repo!")

	fs := osfs.New(directory)
	workFS := memfs.New()
	storageCache := cache.NewObjectLRUDefault()
	stor := filesystem.NewStorage(fs, storageCache)
	r, err := git.Open(stor, workFS)
	if err != nil {
		log.Fatalf("could not open %q: %s", directory, err)
	}

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
		log.Fatalf("could not get branches from %q (%q)", url, directory)
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

	err = makeTar(workFS, "/tmp/perl.tar")
	if err != nil {
		log.Fatalf("error making tar: %s")
	}

}

func makeTar(fs billy.Filesystem, file string) error {
	baseDir := "foo/"
	path := "/"
	info, err := fs.Stat(path)
	if err != nil {
		panic(err)
	}

	header, err := tar.FileInfoHeader(info, path)
	if err != nil {
		panic(err)
	}

	if baseDir != "" {
		header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
	}

	if info.IsDir() {
		header.Name += "/"
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		panic(err)
	}

	if info.IsDir() {
		files, err := fs.ReadDir(source)
		if err != nil {
			panic(err)
		}

		for _, f := range files {
			if err = memFsTarFile(tarWriter, source, baseDir, filepath.Join(source, f.Name()), fs); err != nil {
				panic(err)
			}
		}

		return nil
	}

	if header.Typeflag == tar.TypeReg {
		file, err := fs.Open(path)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		bytesWritten, err := io.CopyN(tarWriter, file, info.Size())
		if err != nil && err != io.EOF {
			panic(err)
		}
	}

	return nil
}
