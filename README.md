# go-version

A go library for generate version information for your application.

`version` module provides some API to help Go Application get it's version
with zero-cost.

After Go 1.15, version information for applications written in Go is
available from its own binary. This is a very useful feature, but I've
noticed that almost no one in the community at large uses it.

I think this is probably because runtime/debug.BuildInfo is harder to use
directly, and the community doesn't have a ready-made library to simplify
this. So I wrote this module to help people easily get the version information
of their Go programs. Of course, based on the go module version control policy
and semantic version.

See also:

* Go Modules Reference: https://go.dev/ref/mod
* Semantic Versioning 2.0.0: https://semver.org/spec/v2.0.0.html
