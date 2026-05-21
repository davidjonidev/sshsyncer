package rsync

import (
	"bufio"
	"strings"
)

// Plan summarises a dry-run output as added/modified/deleted lists.
type Plan struct {
	Added    []string
	Modified []string
	Deleted  []string
}

// ParsePlan reads rsync's itemized output (`-azv --dry-run`) and classifies
// each line.
func ParsePlan(output string) Plan {
	var p Plan
	sc := bufio.NewScanner(strings.NewReader(output))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "*deleting"):
			rest := strings.TrimSpace(strings.TrimPrefix(line, "*deleting"))
			if rest != "" {
				p.Deleted = append(p.Deleted, rest)
			}
		case len(line) > 12 && (line[0] == '>' || line[0] == '<') && line[1] == 'f':
			path := strings.TrimSpace(line[12:])
			if strings.Contains(line[2:11], "+++++++++") {
				p.Added = append(p.Added, path)
			} else {
				p.Modified = append(p.Modified, path)
			}
		}
	}
	return p
}
