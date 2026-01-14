package gunp

import (
	logger "gunp/internal/log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"
)

type GunpRepo struct {
	Path            string
	UnpushedCommits []*object.Commit
}

/*
GunpTUI returns the following:

  - rootDir: string - the root directory
  - discoveryDoneCh: chan bool - a channel that is closed when the discovery is done
  - scanningDoneCh: chan bool - a channel that is closed when the scanning is done
  - walkedPathsCounter: *gunp.Counter - a counter that tracks the walked paths count as they are discovered one by one
  - gitPathsCh: chan string - a channel that stream the git paths as they are discovered one by one
  - gunpReposCh: chan *GunpRepo - a channel that stream the gunp repos as they are discovered one by one
  - err: error - an error if any
*/
func GunpTUI() (string, chan bool, chan bool, *Counter, chan string, chan *GunpRepo, error) {
	concurrencyGlobal := 10 // number of workers for the stats
	discoveryDoneCh := make(chan bool)
	scanningDoneCh := make(chan bool)
	walkedPathsCounter := NewCounter()
	gitPathsCh := make(chan string, 1)
	gunpReposCh := make(chan *GunpRepo, concurrencyGlobal)

	rootDir := rootCwd()

	rawGitPaths := make(chan string, 1)

	go func() {
		defer close(rawGitPaths)
		gitPaths(rootDir, rawGitPaths, walkedPathsCounter)
	}()

	gitPathsChForStats := make(chan string, concurrencyGlobal)
	go func() {
		defer close(gitPathsCh)
		defer close(gitPathsChForStats)
		defer walkedPathsCounter.Close()
		defer close(discoveryDoneCh)
		for gitPath := range rawGitPaths {
			gitPathsCh <- gitPath
			gitPathsChForStats <- gitPath
		}
		// discoveryDoneCh <- true
	}()
	go func() {
		// <-discoveryDoneCh
		// defer close(gunpReposCh)
		defer close(scanningDoneCh)
		gunpStats(gitPathsChForStats, gunpReposCh, concurrencyGlobal)
		// scanningDoneCh <- true
	}()
	return rootDir, discoveryDoneCh, scanningDoneCh, walkedPathsCounter, gitPathsCh, gunpReposCh, nil
}

// main algorithm that recursively explore the current folder and get git status
func gunp() {
	rootDir := rootCwd()
	paths := gitPathsPlain(rootDir)

	slog.Debug("PATHS", "rootDir", rootDir, "paths", paths)

	globalCount := 0

	for _, currGitPath := range paths {
		stats := gitStats(currGitPath)
		// slog.Debug("Git Status by repo", "stats", stats)
		if len(stats.UnpushedCommits) > 0 {
			logger.Get().Print("-", stats.Path, len(stats.UnpushedCommits))
			globalCount = globalCount + len(stats.UnpushedCommits)
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
	return rootPath
}

func gitPathsPlain(rootDir string) []string {
	return gitPaths(rootDir, nil, nil)
}

func gitPaths(rootDir string, gitPathsCh chan string, counter *Counter) []string {
	files, err := os.ReadDir(rootDir)
	if err != nil {
		logger.Get().Error("Read Directory", "rootDir", rootDir, "err", err)
		os.Exit(1)
	}

	if counter != nil {
		counter.Add(1)
	}

	var gitPathsInternal []string

	var pathsToExplore []string

	for _, file := range files {
		if file.IsDir() && file.Name() == ".git" {
			if gitPathsCh != nil {
				gitPathsCh <- rootDir
			}
			gitPathsInternal = append(gitPathsInternal, rootDir)
			continue
		}
		if file.IsDir() && !strings.HasPrefix(file.Name(), ".") {
			pathsToExplore = append(pathsToExplore, fullpath(rootDir, file.Name()))
		}
	}

	if len(pathsToExplore) > 0 {
		for _, cwpath := range pathsToExplore {
			gitPathsInternal = append(gitPathsInternal, gitPaths(cwpath, gitPathsCh, counter)...)
		}
	}

	return gitPathsInternal
}

func fullpath(root string, pathName string) string {
	return filepath.Join(root, pathName)
}

func gunpStats(gitPathsCh chan string, gunpReposCh chan *GunpRepo, numberOfWorkers int) []*GunpRepo {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var gunpRepos []*GunpRepo

	for i := 0; i < numberOfWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for gitPath := range gitPathsCh {
				stats := gitStats(gitPath)
				if gunpReposCh != nil {
					gunpReposCh <- stats
				}
				mu.Lock()
				gunpRepos = append(gunpRepos, stats)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if gunpReposCh != nil {
		close(gunpReposCh)
	}

	return gunpRepos
}

func gitStats(gitDir string) *GunpRepo {
	r, err := git.PlainOpen(gitDir)
	if err != nil {
		logger.Get().Error("Git open repository", "gitDir", gitDir, "err", err)
		// os.Exit(1)
		return &GunpRepo{
			Path:            gitDir,
			UnpushedCommits: []*object.Commit{},
		}
	}

	unpushedCount := GetUnpushedCommits(r)
	logger.Get().Info("UNPUSHED", "gitDir", gitDir, "unpushed commits", len(unpushedCount))

	return &GunpRepo{
		Path:            gitDir,
		UnpushedCommits: unpushedCount,
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
