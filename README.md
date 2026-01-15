# gunp

Recursively scan git repos for unpushed commits with a nice Terminal UI

## Installation

```sh
go install github.com/b3nab/gunp
```

## Usage

```sh
gunp
```

## Demo Fast 1 (1ms)

Stats:
- Total time: **1ms**
- Directories Walked: **10**
- Repositories Discovered/Scanned: **1**
- Unpushed Commits: **11**

<img alt="Welcome to VHS" src="./demo/gunp-demo-fast-1.gif" width="600" />

## Demo Fast 2 (9ms)

Stats:
- Total time: **9ms**
- Directories Walked: **268**
- Repositories Discovered/Scanned: **5**
- Unpushed Commits: **16**

<img alt="Welcome to VHS" src="./demo/gunp-demo-fast-2.gif" width="600" />

## Demo Heavy (44s)

Stats:
- Total time: **44s**
- Directories Walked: **510034**
- Repositories Discovered/Scanned: **120**
- Unpushed Commits: **41**

<img alt="Welcome to VHS" src="./demo/gunp-demo-heavy.gif" width="600" />
