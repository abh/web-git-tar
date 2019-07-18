package main

import (
	"testing"
)

func TestDotPatch(t *testing.T) {

	tests := [][]string{
		[]string{"maint-5.10", "maint-5.10 2013-03-07.22:51:37 e94431af0ada74486afd65207f3e0345fe7485fe v5.10.1-6-ge94431af0a"},
		[]string{"287636082155", "blead 2019-06-20.19:28:35 287636082155028adce267d8e660aedeea514897 v5.31.1"},
		[]string{"05ba7c096a1637", "blead 2019-06-24.10:40:07 05ba7c096a1637812610fe686e02f626fa5a39f0 v5.31.1-96-g05ba7c096a"},
		[]string{"44523d1ffde5f2", "blead 2019-05-22.08:29:58 44523d1ffde5f23de2e13216cdbac46357631904 v5.30.0"},
	}

	gt := GitTar{
		RepoURL:   "git://perl5.git.perl.org/perl.git",
		Directory: "/tmp/perl",
	}
	gt.Setup()

	for _, dpt := range tests {
		r, err := gt.Load()
		if err != nil {
			t.Fatalf("could not load repo: %s", err)
		}
		// w, err := r.Worktree()
		// if err != nil {
		// 	t.Fatalf("could not get worktree: %s", err)
		// }

		pl, err := gt.GetPatchLine(r, dpt[0])
		if err != nil {
			t.Fatalf("error getting pl for %q: %s", dpt[0], err)
		}
		if pl != dpt[1] {
			t.Logf("Wrong patch line for %q\n- Expected: %q\n- Got     : %q", dpt[0], dpt[1], pl)
			t.Fail()
			continue
		}

	}
}
