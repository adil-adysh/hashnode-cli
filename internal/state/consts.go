package state

// Filenames stored under the project StateDir
const (
    StateDir      = ".hashnode"
    SumFile       = "hashnode.sum"
    StageFilename = "hashnode.stage"
    LockFile      = "hashnode.lock"
    ArticlesFile  = "article.yml"
    SeriesFile    = "series.yml"
)

// File and directory permissions used across the project
const (
    FilePerm      = 0644
    DirPerm       = 0755
    SecureDirPerm = 0700
)
