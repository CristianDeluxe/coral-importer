# Contributing

Contributing to the `coral-importer` tool requires a full [Go toolchain](https://go.dev/doc/install).

When you modify files, ensure you that before you commit, you run:

```sh
go generate ./...
```

Which may regenerate files based on updates models.

If you see errors related to $PATH such as:
```sh
common/coral/commentActions.go:1: running "easyjson": exec: "easyjson": executable file not found in $PATH
```
you can fix by exporting the following and adding these to your bash profile and confirming that you have the dependency installed:
```sh
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
go install github.com/mailru/easyjson/easyjson@v0.7.7
```
