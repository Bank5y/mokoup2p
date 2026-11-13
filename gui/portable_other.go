//go:build !windows

package main

import "github.com/Bank5y/mokoup2p/client/ipc"

// startPortable: there is no portable edition outside Windows.
func startPortable(*ipc.Client) bool { return true }

func dependencyMissing() bool { return false }
