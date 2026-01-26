package main

import "github.com/amonks/incrementum/opencode"

func openOpencodeStoreAndRepoPath() (*opencode.Store, string, error) {
	store, err := opencode.Open()
	if err != nil {
		return nil, "", err
	}

	repoPath, err := opencode.RepoPathForWorkingDir()
	if err != nil {
		return nil, "", err
	}

	return store, repoPath, nil
}
