package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"

	// tea "github.com/charmbracelet/bubbletea"

	"gunp/internal/log"
)

func main() {
	logger.Initialize()

	logger.Get().Print("gunp - Git Unpushed")

	logger.Get().Print("By running gunp it will recursively explore all folders starting from the current, and count the unpushed commits of your git repositories.")

	gunp()
}

// main algorithm that recursively explore the current folder and get git status
func gunp() {
	rootDir := rootCwd()
	paths := gitPaths(rootDir)

	slog.Debug("PATHS", "rootDir", rootDir, "paths", paths)

	globalCount := 0

	for _, currGitPath := range paths {
		stats := gitStats(currGitPath)
		// slog.Debug("Git Status by repo", "stats", stats)
		if len(stats.unpushedCommits) > 0 {
			logger.Get().Print("-", stats.path, len(stats.unpushedCommits))
			globalCount = globalCount + len(stats.unpushedCommits)
		}
	}

	logger.Get().Print("Final Stats", "directory", rootDir, "# commits to push", globalCount)

}

func rootCwd() string {
	rootPath, err := os.Getwd()
	if err != nil {
		logger.Get().Error("Get current directory: ", "err", err)
		os.Exit(1)
	}

	// fmt.Printf("Start directory: %s\n", rootPath)

	return rootPath
}

func gitPaths(rootDir string) []string {
	files, err := os.ReadDir(rootDir)
	if err != nil {
		logger.Get().Error("Read Directory", "rootDir", rootDir, "err", err)
		os.Exit(1)
	}

	var gitPathsInternal []string

	var pathsToExplore []string

	for _, file := range files {
		// fmt.Println(file.Name(), file.IsDir())
		if file.IsDir() && file.Name() == ".git" {
			gitPathsInternal = append(gitPathsInternal, rootDir)
			continue
		}
		if file.IsDir() && !strings.HasPrefix(file.Name(), ".") {
			pathsToExplore = append(pathsToExplore, fullpath(rootDir, file.Name()))
		}
	}

	if len(pathsToExplore) > 0 {
		for _, cwpath := range pathsToExplore {
			gitPathsInternal = append(gitPathsInternal, gitPaths(cwpath)...)
		}
	}
	// fmt.Printf("%s\n", gitPathsInternal)

	return gitPathsInternal
}

func fullpath(root string, pathName string) string {
	return filepath.Join(root, pathName)
}

type RepoStat struct {
	path            string
	unpushedCommits []*object.Commit
}

func gitStats(gitDir string) *RepoStat {
	r, err := git.PlainOpen(gitDir)
	if err != nil {
		logger.Get().Error("Git open repository", "gitDir", gitDir, "err", err)
		// os.Exit(1)
		return &RepoStat{
			path:            gitDir,
			unpushedCommits: []*object.Commit{},
		}
	}

	unpushedCount := GetUnpushedCommits(r)
	logger.Get().Info("UNPUSHED", "gitDir", gitDir, "unpushed commits", len(unpushedCount))

	return &RepoStat{
		path:            gitDir,
		unpushedCommits: unpushedCount,
	}
}

func GetUnpushedCommits(repo *git.Repository) []*object.Commit {
	var commits []*object.Commit

	// Get the local HEAD reference
	head, err := repo.Head()
	if err != nil {
		logger.Get().Error("get HEAD err:", "err", err)
		return commits
	}

	config, err := repo.Config()
	if err != nil {
		logger.Get().Error("get CONFIG", "err", err)
		return commits
	}

	var remoteName string
	var stopHash plumbing.Hash // Defaults to ZeroHash (walk all history)

	branchName := head.Name().Short()
	branchConfig := config.Branches[branchName]
	if branchConfig != nil && branchConfig.Remote != "" && branchConfig.Merge != "" {
		// there is a REMOTE branch to track
		remoteName = "refs/remotes/" + branchConfig.Remote + "/" + branchConfig.Merge.Short()
		// remoteName := branchConfig.Remote + "/" + branchConfig.Merge.Short()
	} else {
		remoteName = "refs/remotes/origin/" + branchName
	}

	remoteRef, err := repo.Reference(plumbing.ReferenceName(remoteName), true)
	if err != nil {
		logger.Get().Error("get REMOTE", "remoteName", remoteName, "err", err)
		return commits
		// goto iterCommits
	}
	localCommit, _ := repo.CommitObject(head.Hash())
	remoteCommit, _ := repo.CommitObject(remoteRef.Hash())
	bases, err := localCommit.MergeBase(remoteCommit)
	if err != nil || len(bases) == 0 {
		return commits
	}
	stopHash = bases[0].Hash
	logger.Get().Debug("remoteRef", "remoteName", remoteName, "hash", remoteRef.Hash().String())

	// iterCommits:

	if stopHash == head.Hash() {
		logger.Get().Debug("Branch is behind remote - skipping -", "stopHash", stopHash, "headHash", head.Hash())
		return commits
	}

	cIter, err := repo.Log(&git.LogOptions{
		From: head.Hash(),
		To:   stopHash,
	})
	if err != nil {
		logger.Get().Error("get LOGS", "err", err)
		return commits
	}

	defer cIter.Close()

	iterErr := cIter.ForEach(func(c *object.Commit) error {
		logger.Get().Debug("commit", "hash", c.Hash.String())
		if c.Hash == stopHash {
			return storer.ErrStop
		}
		commits = append(commits, c)
		return nil
	})
	if iterErr != nil {
		logger.Get().Error("iter COMMITS", "err", err)
		return commits
	}

	return commits
}
