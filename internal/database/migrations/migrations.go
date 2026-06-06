package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Files is compiled into the migration binary so deployed code and schema
// changes cannot drift because of an omitted SQL directory.
//
//go:embed *.sql
var Files embed.FS

type Migration struct {
	Version string
	SQL     string
}

func All() ([]Migration, error) {
	entries, err := fs.ReadDir(Files, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		contents, err := Files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		migrations = append(migrations, Migration{Version: entry.Name(), SQL: string(contents)})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	return migrations, nil
}
