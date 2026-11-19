//go:build !linux

package main

// amitty supports Linux only; this undefined name stops other builds.
var _ = amittyBuildsOnLinuxOnly
