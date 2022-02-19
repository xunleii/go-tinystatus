# go-tinystatus

go-tinystatus reimplements [tinystatus](https://github.com/bderenzo/tinystatus) - _a shell script generating a html
status page_ - in Golang.

**Why ?** I really love the concept on _tinystatus_; generates the smallest HTTP page as possible with minimal
dependencies. But, because it still requires some dependencies (`nc`, `curl`, `coreutils` and ... an HTTP server), I
wanted to remove all remaining dependencies.

## Features

* `tinystatus` Parallel checks
* `go-tinystatus` Group several checks by category
* `tinystatus` HTTP, ping, port checks
* `tinystatus` HTTP expected status code (401, ...)
* `tinystatus` Minimal dependencies (curl, nc and coreutils)
* `tinystatus` Easy configuration and customisation
* `tinystatus` Tiny (~1kb) optimized result page
* `tinystatus` Incident history (manual)
* `go-tinystatus` Self-embedded web server
* `go-tinystatus` Automatic update of checks and incidents

### Unavailable features

* `go-tinystatus` cannot handle `ping6` checks
  * But it is possible to use `ping` with an IPv6 target _(cannot force IPv6)_.
* `go-tinystatus` cannot be setup using environment variable
  * But it uses flags instead. See `--help` for more information.

## Demo

Because `go-tinystatus` use the same HTML code that `tinystatus`, you can see a demo on
its `README.md` : [tinystatus](https://github.com/bderenzo/tinystatus)

## Setup

To install go-tinystatus:

* Clone the repository and go to the created directory
* Build the project with `go build .`
* Edit the checks file `checks.csv`
* To add incidents or maintenance, edit `incidents.txt`
* Generate status page `./go-tinystatus > index.html` and serve the page with your favorite web server
* Or run the embedded web server with `./go-tinystatus --daemon`

## Configuration file

The syntax of `checks.csv` file is:

```
Command, Expected Code, Status Text, Host to check, Category
```

Command can be:

* `http` - Check http status
* `ping` - Check ping status
* `port` or `tcp` - Check open port status

There are also `http4`, `http6`, `ping4`, `ping6`, `port4`, `port6` for IPv4 or IPv6 only check.
**Note:** `ping6` is not available on `go-tinystatus`, but you can use `ping` with an IPv6 target.
