# Web Git Tar

Small tooll to build a tar file of a git repository. It has special support
for adding the Perl `.patch` file to the archive.

## TODO

- Add http API to request a specific commit, branch or tag
  - Don't write the file to /tmp/perl.tar, just write it to the user
- Add appropriate Cache-Control headers and put it behind a CDN
- Add http API for browsing available targets?
- Various prototype cleanups
