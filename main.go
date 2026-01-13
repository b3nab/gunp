package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"
	// tea "github.com/charmbracelet/bubbletea"

	"github.com/lmittmann/tint" // Nice colored output
)

const (
	LevelTrace  = slog.Level(-8)
	LevelDev    = slog.Level(-6)
	LevelSilent = slog.Level(100)
	LevelUser   = slog.Level(101)
)

var LevelNames = map[slog.Leveler]string{
	LevelTrace:  "TRACE",
	LevelDev:    "DEV",
	LevelSilent: "SILENT",
	LevelUser:   "",
}

type Logger struct {
	*slog.Logger
}

func (l *Logger) Trace(msg string, args ...any) {
	l.Log(context.TODO(), LevelTrace, msg, args...)
}
func (l *Logger) Dev(msg string, args ...any) {
	l.Log(context.TODO(), LevelDev, msg, args...)
}
func (l *Logger) Print(msg string, args ...any) {
	l.Log(context.TODO(), LevelUser, msg, args...)
}

func NewLogger() *Logger {
	opts := &tint.Options{
		Level:      LevelSilent,
		TimeFormat: time.Kitchen,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)
				if name, ok := LevelNames[level]; ok {
					a.Value = slog.StringValue(name)
				}
			}
			return a
		},
	}

	return &Logger{
		Logger: slog.New(tint.NewHandler(os.Stdout, opts)),
	}
}

// var logLevel = new(slog.LevelVar)
// func log(msg string, data ...any) {
// 	if slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
// 		slog.Debug(msg, data...)
// 	}
// }

var logger *Logger

func main() {
	// logHandler := tint.NewHandler(os.Stdout, &tint.Options{
	// Level:      logLevel,
	// TimeFormat: time.Kitchen,
	// })
	logger = NewLogger()
	logHandler := logger.Handler()
	slog.SetDefault(slog.New(logHandler))

	logger.Print("gunp - Git Unpushed")

	logger.Print("By running gunp it will recursively explore all folders starting from the current, and count the unpushed commits of your git repositories.")

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
			logger.Print("-", stats.path, len(stats.unpushedCommits))
			globalCount = globalCount + len(stats.unpushedCommits)
		}
	}

	logger.Print("Final Stats", "directory", rootDir, "# commits to push", globalCount)

}

func rootCwd() string {
	rootPath, err := os.Getwd()
	if err != nil {
		logger.Error("Get current directory: ", "err", err)
		os.Exit(1)
	}

	// fmt.Printf("Start directory: %s\n", rootPath)

	return rootPath
}

func gitPaths(rootDir string) []string {
	files, err := os.ReadDir(rootDir)
	if err != nil {
		logger.Error("Read Directory", "rootDir", rootDir, "err", err)
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
		logger.Error("Git open repository", "gitDir", gitDir, "err", err)
		// os.Exit(1)
		return &RepoStat{
			path:            gitDir,
			unpushedCommits: []*object.Commit{},
		}
	}

	unpushedCount := GetUnpushedCommits(r)
	logger.Info("UNPUSHED", "gitDir", gitDir, "unpushed commits", len(unpushedCount))

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
		logger.Error("get HEAD err:", "err", err)
		return commits
	}

	config, err := repo.Config()
	if err != nil {
		logger.Error("get CONFIG", "err", err)
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
		logger.Error("get REMOTE", "remoteName", remoteName, "err", err)
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
	logger.Debug("remoteRef", "remoteName", remoteName, "hash", remoteRef.Hash().String())

	// iterCommits:

	if stopHash == head.Hash() {
		logger.Debug("Branch is behind remote - skipping -", "stopHash", stopHash, "headHash", head.Hash())
		return commits
	}

	cIter, err := repo.Log(&git.LogOptions{
		From: head.Hash(),
		To:   stopHash,
	})
	if err != nil {
		logger.Error("get LOGS", "err", err)
		return commits
	}

	defer cIter.Close()

	iterErr := cIter.ForEach(func(c *object.Commit) error {
		logger.Debug("commit", "hash", c.Hash.String())
		if c.Hash == stopHash {
			return storer.ErrStop
		}
		commits = append(commits, c)
		return nil
	})
	if iterErr != nil {
		logger.Error("iter COMMITS", "err", err)
		return commits
	}

	return commits
}
