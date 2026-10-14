package grpcclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"

	"roundtrip/internal/model"
	"roundtrip/internal/sender"
)

// Protos is where descriptors come from. With an empty Config they come from
// server reflection. Otherwise they come from the .proto files only, compiled
// on every call so an edit or a `git pull` is picked up by the next one.
type Protos struct {
	Config model.ProtoConfig
	// Workspace is the directory relative roots and files are resolved against.
	Workspace string
}

// Check compiles the proto files of p and reports what they define.
func Check(ctx context.Context, p Protos, vars map[string]string) (model.ProtoCheck, error) {
	set, err := p.load(ctx, vars)
	if err != nil {
		return model.ProtoCheck{}, err
	}
	return set.check(), nil
}

// protoSet is a compiled proto config.
type protoSet struct {
	// files are the loaded files, without the files they only import.
	files []protoreflect.FileDescriptor
	types anyTypes
}

func (s *protoSet) services() []protoreflect.ServiceDescriptor {
	var out []protoreflect.ServiceDescriptor
	for _, f := range s.files {
		for i := range f.Services().Len() {
			out = append(out, f.Services().Get(i))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FullName() < out[j].FullName() })
	return out
}

func (s *protoSet) check() model.ProtoCheck {
	c := model.ProtoCheck{Files: len(s.files), Services: []string{}, Methods: []string{}}
	for _, svc := range s.services() {
		c.Services = append(c.Services, string(svc.FullName()))
		c.Methods = append(c.Methods, methodNames(svc)...)
	}
	sort.Strings(c.Methods)
	return c
}

func (s *protoSet) service(name string) (protoreflect.ServiceDescriptor, error) {
	for _, svc := range s.services() {
		if string(svc.FullName()) == name {
			return svc, nil
		}
	}
	return nil, fmt.Errorf("service %s is not defined in the loaded proto files, which define: %s", name, list(s.check().Services))
}

// anyTypes finds the message inside a google.protobuf.Any in the loaded files
// and what they import, then in the types compiled into this binary.
type anyTypes struct{ *dynamicpb.Types }

func (t anyTypes) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	if mt, err := t.Types.FindMessageByName(name); err == nil {
		return mt, nil
	}
	return protoregistry.GlobalTypes.FindMessageByName(name)
}

func (t anyTypes) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	if mt, err := t.Types.FindMessageByURL(url); err == nil {
		return mt, nil
	}
	return protoregistry.GlobalTypes.FindMessageByURL(url)
}

// expand resolves the variables and a leading ~ of a configured path. Both
// separators are accepted on every system, so a config committed from Windows
// works on macOS and the other way round. The result may still be relative.
func expand(kind, raw string, vars map[string]string) (string, error) {
	s := strings.TrimSpace(sender.Resolve(raw, vars))
	if names := sender.Unresolved(s); len(names) > 0 {
		return "", fmt.Errorf("proto %s %q: variable %s is not set (set it in the environment)", kind, raw, names[0])
	}
	s = strings.ReplaceAll(s, `\`, "/")
	if s == "~" || strings.HasPrefix(s, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("proto %s %q: %w", kind, raw, err)
		}
		s = home + s[1:]
	}
	return filepath.Clean(filepath.FromSlash(s)), nil
}

// under returns path relative to root, as an import name, when it lies below root.
func under(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// source reads the files the compiler asks for, by import name, from the roots.
type source struct {
	roots []string

	mu    sync.Mutex
	paths map[string]string // import name -> the file it was read from
}

type missingImport struct {
	name  string
	roots []string
}

func (e *missingImport) Error() string {
	return fmt.Sprintf("import %q not found under the import roots (searched: %s)", e.name, strings.Join(e.roots, ", "))
}

func (s *source) FindFileByPath(name string) (protocompile.SearchResult, error) {
	for _, root := range s.roots {
		path := filepath.Join(root, filepath.FromSlash(name))
		if !isFile(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return protocompile.SearchResult{}, err
		}
		s.mu.Lock()
		s.paths[name] = path
		s.mu.Unlock()
		return protocompile.SearchResult{Source: bytes.NewReader(data)}, nil
	}
	return protocompile.SearchResult{}, &missingImport{name, s.roots}
}

// describe turns a compiler error into "<file on disk>:line:col: message".
func (s *source) describe(err error) string {
	msg := err.Error()
	var missing *missingImport
	if errors.As(err, &missing) {
		msg = missing.Error()
	}
	var at reporter.ErrorWithPos
	if !errors.As(err, &at) {
		return msg
	}
	if missing == nil {
		msg = at.Unwrap().Error()
	}
	pos := at.GetPosition()
	s.mu.Lock()
	file, ok := s.paths[pos.Filename]
	s.mu.Unlock()
	if !ok {
		file = pos.Filename
	}
	if pos.Line == 0 {
		return file + ": " + msg
	}
	return fmt.Sprintf("%s:%d:%d: %s", file, pos.Line, pos.Col, msg)
}

// plan works out the import roots and the import names of the files to load.
func (p Protos) plan(vars map[string]string) (roots, names []string, err error) {
	for _, raw := range p.Config.Roots {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		root, err := expand("root", raw, vars)
		if err != nil {
			return nil, nil, err
		}
		if !filepath.IsAbs(root) {
			root = filepath.Join(p.Workspace, root)
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			if err == nil {
				return nil, nil, fmt.Errorf("proto root %q is not a directory: %s", raw, root)
			}
			return nil, nil, fmt.Errorf("proto root %q does not exist: %s", raw, root)
		}
		if !slices.Contains(roots, root) {
			roots = append(roots, root)
		}
	}
	explicit := len(roots) > 0
	add := func(name string) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	listed := false
files:
	for _, raw := range p.Config.Files {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		listed = true
		file, err := expand("file", raw, vars)
		if err != nil {
			return nil, nil, err
		}
		abs := file
		if !filepath.IsAbs(file) {
			abs = filepath.Join(p.Workspace, file)
		}
		if !explicit {
			// Without roots a file is loaded by its base name from its own directory.
			if !isFile(abs) {
				return nil, nil, fmt.Errorf("proto file %q does not exist: %s", raw, abs)
			}
			if dir := filepath.Dir(abs); !slices.Contains(roots, dir) {
				roots = append(roots, dir)
			}
			add(filepath.Base(abs))
			continue
		}
		// Like protoc: relative to an import root first.
		if !filepath.IsAbs(file) {
			for _, root := range roots {
				if name, ok := under(root, filepath.Join(root, file)); ok && isFile(filepath.Join(root, file)) {
					add(name)
					continue files
				}
			}
		}
		// Then a full or workspace-relative path that lies under a root.
		if !isFile(abs) {
			return nil, nil, fmt.Errorf("proto file %q not found under any import root (searched: %s)", raw, strings.Join(roots, ", "))
		}
		for _, root := range roots {
			if name, ok := under(root, abs); ok {
				add(name)
				continue files
			}
		}
		return nil, nil, fmt.Errorf("proto file %q (%s) is not under any import root (%s): add its directory to the roots", raw, abs, strings.Join(roots, ", "))
	}
	if listed || !explicit {
		return roots, names, nil
	}

	// Roots only: every .proto below them. A file reachable through two roots
	// is loaded once, under the first.
	seen := map[string]bool{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path != root && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if name, ok := under(root, path); ok && strings.HasSuffix(d.Name(), ".proto") && d.Type().IsRegular() && !seen[path] {
				seen[path] = true
				add(name)
			}
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("proto root %s: %w", root, err)
		}
	}
	if len(names) == 0 {
		return nil, nil, fmt.Errorf("no .proto files found under the import roots (searched: %s)", strings.Join(roots, ", "))
	}
	return roots, names, nil
}

const maxCompileErrors = 10

func (p Protos) load(ctx context.Context, vars map[string]string) (*protoSet, error) {
	roots, names, err := p.plan(vars)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, errors.New("the proto config has no roots and no files")
	}
	src := &source{roots: roots, paths: map[string]string{}}
	var mu sync.Mutex
	var problems []string
	note := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if msg := src.describe(err); !slices.Contains(problems, msg) {
			problems = append(problems, msg)
		}
	}
	compiler := protocompile.Compiler{
		// Well-known types (google/protobuf/*.proto) are built in; a copy
		// under a root wins, as with protoc.
		Resolver: protocompile.WithStandardImports(src),
		// Keep going after an error so all of them are reported at once.
		Reporter: reporter.NewReporter(func(err reporter.ErrorWithPos) error {
			note(err)
			return nil
		}, nil),
	}
	files, err := compiler.Compile(ctx, names...)
	if err != nil {
		if !errors.Is(err, reporter.ErrInvalidSource) {
			note(err)
		}
		if len(problems) == 0 {
			return nil, err
		}
		sort.Strings(problems)
		if more := len(problems) - maxCompileErrors; more > 0 {
			problems = append(problems[:maxCompileErrors], fmt.Sprintf("... and %d more", more))
		}
		return nil, errors.New(strings.Join(problems, "\n"))
	}

	set := &protoSet{}
	// Two unrelated files may define the same name; the first one wins when
	// resolving an Any rather than failing the whole load.
	registry := &protoregistry.Files{}
	var register func(f protoreflect.FileDescriptor)
	register = func(f protoreflect.FileDescriptor) {
		if _, err := registry.FindFileByPath(f.Path()); err == nil {
			return
		}
		registry.RegisterFile(f)
		for i := range f.Imports().Len() {
			register(f.Imports().Get(i).FileDescriptor)
		}
	}
	for _, f := range files {
		set.files = append(set.files, f)
		register(f)
	}
	set.types = anyTypes{dynamicpb.NewTypes(registry)}
	return set, nil
}
