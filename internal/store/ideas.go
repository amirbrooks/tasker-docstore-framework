package store

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	IdeaScopeRoot    = "root"
	IdeaScopeProject = "project"
	IdeaScopeAll     = "all"
)

type IdeaMeta struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Project   string     `json:"project"`
	Tags      []string   `json:"tags"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type Idea struct {
	IdeaMeta `json:",inline"`
	Path     string `json:"path"`
	Body     string `json:"-"`
}

// IdeaMatchConflictError provides details when a selector matches multiple ideas.
// It still satisfies errors.Is(err, ErrConflict).
type IdeaMatchConflictError struct {
	Reason  string
	Matches []Idea
}

func (e *IdeaMatchConflictError) Error() string {
	if e == nil || strings.TrimSpace(e.Reason) == "" {
		return "conflict"
	}
	return "conflict: " + e.Reason
}

func (e *IdeaMatchConflictError) Is(target error) bool {
	return target == ErrConflict
}

type AddIdeaInput struct {
	Title   string
	Project string
	Tags    []string
	Body    string
}

type IdeaListFilter struct {
	Project string
	Scope   string
	Tag     string
	Search  string
}

type IdeaSelectorFilter struct {
	Project string
	Scope   string
	Match   string
}

type ideaDir struct {
	Project string
	Path    string
}

type ideaPath struct {
	Project string
	Path    string
}

func (w *Workspace) AddIdea(in AddIdeaInput) (*Idea, error) {
	title := normalizeIdeaTitle(strings.TrimSpace(in.Title))
	if title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalid)
	}
	projectName := strings.TrimSpace(in.Project)
	projectSlug := ""
	if projectName != "" {
		project, err := w.CreateProject(projectName)
		if err != nil {
			return nil, err
		}
		projectSlug = project.Slug
	}
	body := strings.TrimRight(in.Body, "\n")
	tags := normalizeIdeaTags(in.Tags)
	id := "idea_" + newULID()
	filename := fmt.Sprintf("%s__%s.md", id, slugify(title))
	dir := w.rootIdeasDir()
	if projectSlug != "" {
		dir = w.projectIdeasDir(projectSlug)
	}
	path := filepath.Join(dir, filename)
	if err := writeIdeaFile(path, title, tags, body); err != nil {
		return nil, err
	}
	tags = inferIdeaTags(title, body, tags)
	now := timeNow()
	idea := &Idea{
		IdeaMeta: IdeaMeta{
			ID:        id,
			Title:     title,
			Project:   projectSlug,
			Tags:      tags,
			CreatedAt: &now,
			UpdatedAt: &now,
		},
		Path: path,
		Body: body,
	}
	return idea, nil
}

func (w *Workspace) GetIdeaBySelectorFiltered(selector string, filter IdeaSelectorFilter) (*Idea, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, ErrInvalid
	}
	matches, err := w.resolveIdeaSelectorCandidates(selector, filter)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, ErrNotFound
	}
	if len(matches) == 1 {
		match := matches[0]
		return &match, nil
	}
	return nil, &IdeaMatchConflictError{Reason: "selector", Matches: matches}
}

func (w *Workspace) ResolveIdeas(selector string, filter IdeaSelectorFilter) ([]Idea, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, ErrInvalid
	}
	matches, err := w.resolveIdeaSelectorCandidates(selector, filter)
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func (w *Workspace) DeleteIdea(idea *Idea) error {
	if idea == nil || strings.TrimSpace(idea.Path) == "" {
		return ErrInvalid
	}
	return os.Remove(idea.Path)
}

func (w *Workspace) AddIdeaNote(idea *Idea, note string) (*Idea, error) {
	if idea == nil || strings.TrimSpace(idea.Path) == "" {
		return nil, ErrInvalid
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, fmt.Errorf("%w: note is required", ErrInvalid)
	}
	current, err := readIdeaFile(idea.Path, idea.Project)
	if err != nil {
		return nil, err
	}
	now := timeNow()
	entry := fmt.Sprintf("- %s — %s\n", now.Format(time.RFC3339), note)
	body := strings.TrimRight(current.Body, "\n")
	if body == "" {
		body = entry
	} else {
		body = body + "\n" + entry
	}
	if err := writeIdeaFile(current.Path, current.Title, current.Tags, body); err != nil {
		return nil, err
	}
	current.Body = body
	current.Tags = inferIdeaTags(current.Title, body, current.Tags)
	current.UpdatedAt = &now
	return current, nil
}

func (w *Workspace) ListIdeas(f IdeaListFilter) ([]Idea, error) {
	filter := normalizeIdeaListFilter(f)
	paths, err := w.ideaPaths(filter.Scope, filter.Project)
	if err != nil {
		return nil, err
	}
	var out []Idea
	for _, p := range paths {
		idea, err := readIdeaFile(p.Path, p.Project)
		if err != nil {
			continue
		}
		if filter.Tag != "" && !containsString(idea.Tags, filter.Tag) {
			continue
		}
		if filter.Search != "" {
			q := strings.ToLower(filter.Search)
			if !strings.Contains(strings.ToLower(idea.Title), q) && !strings.Contains(strings.ToLower(idea.Body), q) {
				continue
			}
		}
		out = append(out, *idea)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt != nil && out[j].UpdatedAt != nil && !out[i].UpdatedAt.Equal(*out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(*out[j].UpdatedAt)
		}
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		if out[i].Title != out[j].Title {
			return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (i *Idea) RenderHuman() string {
	if i == nil {
		return ""
	}
	var b strings.Builder
	title := strings.TrimSpace(i.Title)
	if title == "" {
		title = "(untitled)"
	}
	b.WriteString(title + "\n")
	if i.Project == "" {
		b.WriteString("Scope: root\n")
	} else {
		b.WriteString("Project: " + i.Project + "\n")
	}
	if len(i.Tags) > 0 {
		b.WriteString("Tags: " + strings.Join(i.Tags, ", ") + "\n")
	}
	b.WriteString("\n")
	if strings.TrimSpace(i.Body) != "" {
		b.WriteString(strings.TrimRight(i.Body, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

func (w *Workspace) resolveIdeaSelectorCandidates(selector string, filter IdeaSelectorFilter) ([]Idea, error) {
	filter = normalizeIdeaSelectorFilter(filter)
	if isLikelyIdeaIDSelector(selector) {
		matches, err := w.findIdeasByPrefixFiltered(selector, filter)
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 {
			return matches, nil
		}
		return w.findIdeasByMatchMode(selector, filter)
	}
	matches, err := w.findIdeasByMatchMode(selector, filter)
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		return matches, nil
	}
	return w.findIdeasByPrefixFiltered(selector, filter)
}

func normalizeIdeaScope(scope string, project string) string {
	scope = strings.TrimSpace(strings.ToLower(scope))
	switch scope {
	case IdeaScopeRoot, IdeaScopeProject, IdeaScopeAll:
		return scope
	}
	if strings.TrimSpace(project) != "" {
		return IdeaScopeProject
	}
	return IdeaScopeRoot
}

func normalizeIdeaListFilter(filter IdeaListFilter) IdeaListFilter {
	project := strings.TrimSpace(filter.Project)
	if project != "" {
		project = slugifyOrDefault(project, project)
	}
	scope := normalizeIdeaScope(filter.Scope, project)
	tag := strings.TrimSpace(filter.Tag)
	return IdeaListFilter{
		Project: project,
		Scope:   scope,
		Tag:     tag,
		Search:  strings.TrimSpace(filter.Search),
	}
}

func normalizeIdeaSelectorFilter(filter IdeaSelectorFilter) IdeaSelectorFilter {
	project := strings.TrimSpace(filter.Project)
	if project != "" {
		project = slugifyOrDefault(project, project)
	}
	scope := normalizeIdeaScope(filter.Scope, project)
	match := normalizeMatchMode(filter.Match)
	return IdeaSelectorFilter{
		Project: project,
		Scope:   scope,
		Match:   match,
	}
}

func (w *Workspace) ideaPaths(scope string, project string) ([]ideaPath, error) {
	dirs, err := w.ideaDirs(scope, project)
	if err != nil {
		return nil, err
	}
	var out []ideaPath
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir.Path, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !isIdeaFile(d.Name()) {
				return nil
			}
			out = append(out, ideaPath{Project: dir.Project, Path: path})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (w *Workspace) ideaDirs(scope string, project string) ([]ideaDir, error) {
	scope = normalizeIdeaScope(scope, project)
	project = strings.TrimSpace(project)
	if project != "" {
		project = slugifyOrDefault(project, project)
	}
	switch scope {
	case IdeaScopeRoot:
		return []ideaDir{{Path: w.rootIdeasDir(), Project: ""}}, nil
	case IdeaScopeProject:
		if project == "" {
			return nil, ErrInvalid
		}
		return []ideaDir{{Path: w.projectIdeasDir(project), Project: project}}, nil
	case IdeaScopeAll:
		dirs := []ideaDir{{Path: w.rootIdeasDir(), Project: ""}}
		if project != "" {
			dirs = append(dirs, ideaDir{Path: w.projectIdeasDir(project), Project: project})
			return dirs, nil
		}
		projects, err := w.ListProjects()
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			dirs = append(dirs, ideaDir{Path: w.projectIdeasDir(p.Slug), Project: p.Slug})
		}
		return dirs, nil
	default:
		return []ideaDir{{Path: w.rootIdeasDir(), Project: ""}}, nil
	}
}

func (w *Workspace) findIdeasByMatchMode(selector string, filter IdeaSelectorFilter) ([]Idea, error) {
	switch filter.Match {
	case MatchAuto:
		selector = strings.TrimSpace(selector)
		if selector == "" {
			return nil, nil
		}
		matches, err := w.findIdeasByTitleExactFiltered(selector, filter)
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 {
			return matches, nil
		}
		matches, err = w.findIdeasByTitlePrefixFiltered(selector, filter)
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 {
			return matches, nil
		}
		matches, err = w.findIdeasByTitleContainsFiltered(selector, filter)
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 {
			return matches, nil
		}
		return w.findIdeasBySearchFiltered(selector, filter)
	case MatchSearch:
		return w.findIdeasBySearchFiltered(selector, filter)
	case MatchPrefix:
		return w.findIdeasByTitlePrefixFiltered(selector, filter)
	case MatchContains:
		return w.findIdeasByTitleContainsFiltered(selector, filter)
	case MatchExact:
		return w.findIdeasByTitleExactFiltered(selector, filter)
	default:
		return w.findIdeasByTitleExactFiltered(selector, filter)
	}
}

func (w *Workspace) findIdeasByTitleExactFiltered(selector string, filter IdeaSelectorFilter) ([]Idea, error) {
	ideas, err := w.loadIdeas(filter)
	if err != nil {
		return nil, err
	}
	var matches []Idea
	needle := strings.ToLower(strings.TrimSpace(selector))
	for _, idea := range ideas {
		if strings.ToLower(strings.TrimSpace(idea.Title)) == needle {
			matches = append(matches, idea)
		}
	}
	return matches, nil
}

func (w *Workspace) findIdeasByTitlePrefixFiltered(selector string, filter IdeaSelectorFilter) ([]Idea, error) {
	ideas, err := w.loadIdeas(filter)
	if err != nil {
		return nil, err
	}
	var matches []Idea
	needle := strings.ToLower(strings.TrimSpace(selector))
	for _, idea := range ideas {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(idea.Title)), needle) {
			matches = append(matches, idea)
		}
	}
	return matches, nil
}

func (w *Workspace) findIdeasByTitleContainsFiltered(selector string, filter IdeaSelectorFilter) ([]Idea, error) {
	ideas, err := w.loadIdeas(filter)
	if err != nil {
		return nil, err
	}
	var matches []Idea
	needle := strings.ToLower(strings.TrimSpace(selector))
	for _, idea := range ideas {
		if strings.Contains(strings.ToLower(strings.TrimSpace(idea.Title)), needle) {
			matches = append(matches, idea)
		}
	}
	return matches, nil
}

func (w *Workspace) findIdeasBySearchFiltered(selector string, filter IdeaSelectorFilter) ([]Idea, error) {
	ideas, err := w.loadIdeas(filter)
	if err != nil {
		return nil, err
	}
	var matches []Idea
	needle := strings.ToLower(strings.TrimSpace(selector))
	for _, idea := range ideas {
		if strings.Contains(strings.ToLower(idea.Title), needle) || strings.Contains(strings.ToLower(idea.Body), needle) {
			matches = append(matches, idea)
		}
	}
	return matches, nil
}

func (w *Workspace) findIdeasByPrefixFiltered(prefix string, filter IdeaSelectorFilter) ([]Idea, error) {
	paths, err := w.ideaPaths(filter.Scope, filter.Project)
	if err != nil {
		return nil, err
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, nil
	}
	needle := strings.ToUpper(prefix)
	var matches []Idea
	for _, p := range paths {
		idea, err := readIdeaFile(p.Path, p.Project)
		if err != nil {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(idea.ID), needle) {
			matches = append(matches, *idea)
		}
	}
	sortIdeaMatches(matches)
	return matches, nil
}

func (w *Workspace) loadIdeas(filter IdeaSelectorFilter) ([]Idea, error) {
	paths, err := w.ideaPaths(filter.Scope, filter.Project)
	if err != nil {
		return nil, err
	}
	var ideas []Idea
	for _, p := range paths {
		idea, err := readIdeaFile(p.Path, p.Project)
		if err != nil {
			continue
		}
		ideas = append(ideas, *idea)
	}
	return ideas, nil
}

func sortIdeaMatches(matches []Idea) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Project != matches[j].Project {
			return matches[i].Project < matches[j].Project
		}
		if matches[i].Title != matches[j].Title {
			return strings.ToLower(matches[i].Title) < strings.ToLower(matches[j].Title)
		}
		return matches[i].ID < matches[j].ID
	})
}

func isIdeaFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".txt":
		return true
	default:
		return false
	}
}

func ideaIDFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.HasPrefix(strings.ToLower(base), "idea_") {
		parts := strings.SplitN(base, "__", 2)
		return parts[0]
	}
	return base
}

func ideaTitleFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.HasPrefix(strings.ToLower(base), "idea_") {
		base = base[len("idea_"):]
	}
	if parts := strings.SplitN(base, "__", 2); len(parts) == 2 {
		base = parts[1]
	}
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.TrimSpace(base)
	return base
}

func formatIdeaContent(title string, tags []string, body string) string {
	var b strings.Builder
	title = normalizeIdeaTitle(title)
	if title == "" {
		title = "(untitled)"
	}
	b.WriteString(title)
	b.WriteString("\n")
	if len(tags) > 0 {
		b.WriteString("tags: ")
		b.WriteString(strings.Join(tags, ", "))
		b.WriteString("\n")
	}
	body = strings.TrimRight(body, "\n")
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	return b.String()
}

func writeIdeaFile(path string, title string, tags []string, body string) error {
	tags = inferIdeaTags(title, body, tags)
	content := formatIdeaContent(title, tags, body)
	return atomicWriteFile(path, []byte(content), 0o644)
}

// ParseIdeaContent exposes the idea parser for CLI stdin capture.
func ParseIdeaContent(text string) (string, []string, string) {
	return parseIdeaContent(text)
}

func readIdeaFile(path string, project string) (*Idea, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	title, tags, body := parseIdeaContent(string(b))
	if title == "" {
		title = ideaTitleFromFilename(filepath.Base(path))
	}
	id := ideaIDFromFilename(filepath.Base(path))
	info, err := os.Stat(path)
	var modTime time.Time
	if err == nil {
		modTime = info.ModTime().UTC()
	} else {
		modTime = timeNow()
	}
	created := modTime
	updated := modTime
	return &Idea{
		IdeaMeta: IdeaMeta{
			ID:        id,
			Title:     title,
			Project:   project,
			Tags:      dedupeStrings(tags),
			CreatedAt: &created,
			UpdatedAt: &updated,
		},
		Path: path,
		Body: body,
	}, nil
}

func parseIdeaContent(text string) (string, []string, string) {
	s := strings.ReplaceAll(text, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	title := ""
	titleIndex := -1
	var tags []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isIdeaTagsLine(trimmed) {
			tags = append(tags, parseTagsLine(trimmed)...)
			continue
		}
		title = normalizeIdeaTitle(trimmed)
		titleIndex = i
		break
	}
	if titleIndex == -1 {
		return "", tags, strings.TrimRight(s, "\n")
	}
	i := titleIndex + 1
	for i < len(lines) {
		if isIdeaTagsLine(lines[i]) {
			tags = append(tags, parseTagsLine(lines[i])...)
			i++
			continue
		}
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		break
	}
	body := strings.TrimRight(strings.Join(lines[i:], "\n"), "\n")
	tags = append(tags, extractIdeaInlineTags(title)...)
	tags = append(tags, extractIdeaInlineTags(body)...)
	return title, dedupeStrings(tags), body
}

func normalizeIdeaTitle(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "title:") {
		return strings.TrimSpace(line[len("title:"):])
	}
	if strings.HasPrefix(line, "#") {
		line = strings.TrimLeft(line, "#")
		return strings.TrimSpace(line)
	}
	return line
}

func isIdeaTagsLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "tags:") || strings.HasPrefix(lower, "tag:")
}

func parseTagsLine(line string) []string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}
	idx := strings.Index(trimmed, ":")
	if idx == -1 {
		return nil
	}
	value := strings.TrimSpace(trimmed[idx+1:])
	return parseTagList(value)
}

func parseTagList(value string) []string {
	value = strings.ReplaceAll(value, ",", " ")
	fields := strings.Fields(value)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = cleanIdeaTag(f)
		if f == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}

func normalizeIdeaTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = cleanIdeaTag(tag)
		if tag == "" {
			continue
		}
		out = append(out, tag)
	}
	return dedupeStrings(out)
}

func cleanIdeaTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	tag = strings.TrimLeft(tag, "#@+")
	return strings.TrimSpace(tag)
}

func inferIdeaTags(title string, body string, explicit []string) []string {
	out := append([]string{}, explicit...)
	out = append(out, extractIdeaInlineTags(title)...)
	out = append(out, extractIdeaInlineTags(body)...)
	return dedupeStrings(out)
}

func extractIdeaInlineTags(text string) []string {
	var tags []string
	s := strings.ReplaceAll(text, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	inFence := false
	fence := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			if !inFence {
				inFence = true
				fence = trimmed[:3]
			} else if fence == "" || strings.HasPrefix(trimmed, fence) {
				inFence = false
				fence = ""
			}
			continue
		}
		if inFence {
			continue
		}
		if isIdeaHeadingLine(line) {
			continue
		}
		tags = append(tags, extractIdeaTokensFromLine(line)...)
	}
	return tags
}

func isIdeaHeadingLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	if i < len(trimmed) && trimmed[i] == ' ' {
		return true
	}
	return false
}

func extractIdeaTokensFromLine(line string) []string {
	var tags []string
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch != '#' && ch != '@' {
			continue
		}
		if i > 0 && isIdeaTagChar(line[i-1]) {
			continue
		}
		if i+1 >= len(line) || !isIdeaTagChar(line[i+1]) {
			continue
		}
		j := i + 1
		for j < len(line) && isIdeaTagChar(line[j]) {
			j++
		}
		tag := line[i+1 : j]
		if tag != "" {
			tags = append(tags, tag)
		}
		i = j - 1
	}
	return tags
}

func isIdeaTagChar(b byte) bool {
	if b >= 'a' && b <= 'z' {
		return true
	}
	if b >= 'A' && b <= 'Z' {
		return true
	}
	if b >= '0' && b <= '9' {
		return true
	}
	if b == '-' || b == '_' {
		return true
	}
	return false
}

func isLikelyIdeaIDSelector(selector string) bool {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return false
	}
	lower := strings.ToLower(selector)
	if strings.HasPrefix(lower, "idea_") {
		return true
	}
	if len(selector) < 8 {
		return false
	}
	allowed := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	hasDigit := false
	for _, r := range strings.ToUpper(selector) {
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
		if !strings.ContainsRune(allowed, r) {
			return false
		}
	}
	return hasDigit
}

func (w *Workspace) rootIdeasDir() string {
	return filepath.Join(w.Root, "ideas")
}

func (w *Workspace) projectIdeasDir(projectSlug string) string {
	return filepath.Join(w.Root, "projects", projectSlug, "ideas")
}
