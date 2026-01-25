// Package workspace manages a pool of jujutsu workspaces.
//
// This package provides functionality to acquire, release, and manage jujutsu
// workspaces from a shared pool. It's designed for scenarios where multiple
// processes need concurrent access to independent working copies of the same
// repository.
//
// # Basic Usage
//
// Create a pool and acquire a workspace:
//
//	pool, err := workspace.Open()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	wsPath, err := pool.Acquire("/path/to/repo", workspace.AcquireOptions{
//	    Rev: "main",
//	    Purpose: "feature work",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.Release(wsPath)
//
//	// Use the workspace at wsPath...
//
// # Configuration
//
// Repositories can include a incrementum.toml file to configure workspace behavior:
//
//	[workspace]
//	on-create = ["npm install"]  # Run every time workspace is acquired
//
// # Storage
//
// By default, workspaces are stored in ~/.local/share/incrementum/workspaces/ and
// state is stored in ~/.local/state/incrementum/. These locations follow the XDG
// Base Directory Specification.
//
// # Concurrency
//
// The pool uses file locking to safely handle concurrent access from multiple
// processes. Workspaces remain acquired until they are released.
package workspace
