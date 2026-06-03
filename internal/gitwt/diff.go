package gitwt

import (
	"os/exec"
	"strings"
)

// DiffLine is one line of a hunk: T is "add" | "del" | "ctx".
type DiffLine struct {
	T    string `json:"t"`
	Text string `json:"text"`
}

// DiffHunk is a contiguous change block with its @@ header.
type DiffHunk struct {
	Header string     `json:"header"`
	Lines  []DiffLine `json:"lines"`
}

// DiffFile is a changed file with its status and hunks.
type DiffFile struct {
	Path   string     `json:"path"`
	Status string     `json:"status"` // modified | added | deleted | renamed
	Add    int        `json:"add"`
	Del    int        `json:"del"`
	Hunks  []DiffHunk `json:"hunks"`
}

// Diff is the result shown by the worktree diff viewer.
type Diff struct {
	Mode  string     `json:"mode"` // "uncommitted" | "last_commit"
	Ref   string     `json:"ref"`  // e.g. "a1b2c3d" or "a1b2c3d..d4e5f6a"
	Sub   string     `json:"sub"`  // subtitle (commit subject / "working tree")
	Files []DiffFile `json:"files"`
}

// maxDiffLines caps total emitted lines so a huge diff can't blow up the payload.
const maxDiffLines = 8000

// WorktreeDiff returns the diff for a worktree: uncommitted changes vs HEAD when
// the working tree is dirty, otherwise HEAD~1..HEAD. Untracked files are
// included (as added) in the uncommitted case.
func WorktreeDiff(wt string) (Diff, error) {
	dirty := strings.TrimSpace(run(wt, "status", "--porcelain")) != ""
	head := strings.TrimSpace(run(wt, "rev-parse", "--short", "HEAD"))

	if dirty {
		d := Diff{Mode: "uncommitted", Ref: head, Sub: "working tree vs HEAD"}
		d.Files = parseUnified(run(wt, "diff", "HEAD"))
		// untracked files: show each as an added file
		for _, p := range splitLines(run(wt, "ls-files", "--others", "--exclude-standard")) {
			if p == "" {
				continue
			}
			ut := parseUnified(run(wt, "diff", "--no-index", "--", "/dev/null", p))
			for i := range ut {
				ut[i].Status = "added"
				ut[i].Path = p
			}
			d.Files = append(d.Files, ut...)
		}
		return d, nil
	}

	// clean tree: last commit vs its parent
	prev := strings.TrimSpace(run(wt, "rev-parse", "--short", "HEAD~1"))
	if prev == "" {
		// single commit (no parent)
		return Diff{Mode: "last_commit", Ref: head, Sub: "initial commit"}, nil
	}
	sub := strings.TrimSpace(run(wt, "log", "-1", "--format=%s"))
	d := Diff{Mode: "last_commit", Ref: prev + ".." + head, Sub: sub}
	d.Files = parseUnified(run(wt, "diff", "HEAD~1", "HEAD"))
	return d, nil
}

func run(wt string, args ...string) string {
	out, _ := exec.Command("git", append([]string{"-C", wt}, args...)...).Output()
	return string(out)
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// parseUnified turns `git diff` unified output into DiffFiles.
func parseUnified(text string) []DiffFile {
	var files []DiffFile
	var cur *DiffFile
	var hunk *DiffHunk
	total := 0
	flush := func() {
		if cur != nil {
			if hunk != nil {
				cur.Hunks = append(cur.Hunks, *hunk)
				hunk = nil
			}
			files = append(files, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			cur = &DiffFile{Status: "modified", Path: gitPath(line)}
		case cur == nil:
			// preamble before first file
			continue
		case strings.HasPrefix(line, "new file mode"):
			cur.Status = "added"
		case strings.HasPrefix(line, "deleted file mode"):
			cur.Status = "deleted"
		case strings.HasPrefix(line, "rename from ") || strings.HasPrefix(line, "rename to "):
			cur.Status = "renamed"
		case strings.HasPrefix(line, "--- "):
			// ignore
		case strings.HasPrefix(line, "+++ "):
			if p := pathFromMarker(line); p != "" {
				cur.Path = p
			}
		case strings.HasPrefix(line, "@@"):
			if hunk != nil {
				cur.Hunks = append(cur.Hunks, *hunk)
			}
			hunk = &DiffHunk{Header: hunkHeader(line)}
		case hunk != nil && total < maxDiffLines && (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ")):
			t := "ctx"
			switch line[0] {
			case '+':
				t, cur.Add = "add", cur.Add+1
			case '-':
				t, cur.Del = "del", cur.Del+1
			}
			hunk.Lines = append(hunk.Lines, DiffLine{T: t, Text: line[1:]})
			total++
		case hunk != nil && strings.HasPrefix(line, "\\"):
			// "\ No newline at end of file" — skip
		}
	}
	flush()
	return files
}

// gitPath extracts the b-side path from a `diff --git a/x b/x` header.
func gitPath(line string) string {
	rest := strings.TrimPrefix(line, "diff --git ")
	// split on " b/" to get the new path (handles spaces in a-path poorly but ok)
	if i := strings.Index(rest, " b/"); i >= 0 {
		return strings.TrimPrefix(rest[i+1:], "b/")
	}
	return rest
}

// pathFromMarker extracts the path from a "+++ b/path" marker.
func pathFromMarker(line string) string {
	p := strings.TrimSpace(line[3:])
	if p == "/dev/null" {
		return ""
	}
	p = strings.TrimPrefix(p, "b/")
	p = strings.TrimPrefix(p, "a/")
	return p
}

// hunkHeader returns the "@@ -a,b +c,d @@" portion (and any trailing context).
func hunkHeader(line string) string {
	if i := strings.Index(line[2:], "@@"); i >= 0 {
		return strings.TrimRight(line[:i+4], " ")
	}
	return line
}
