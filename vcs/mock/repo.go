package mock

import "errors"

// Repo mocking for unit testing.
// Intended to mock behaviour of the git.Repo implementation.
type Repo struct {
}

// Minimal methods which implement backing.Repo interface.
// All methods reurn the default "zero" values except where fleshed out.

func (mockrepo *Repo) Sync() error {
	return nil
}

func (mockrepo *Repo) BuildQueries() ([]string, error) {
	return []string{}, nil
}

func (mockrepo *Repo) ReadFile(filename, commitID string) (string, error) {
	return "", nil
}

func (mockrepo *Repo) CurrentCommitId() (string, error) {
	return "", nil
}

func (mockrepo *Repo) Patches() ([]string, error) {
	return nil, nil
}

func (mockrepo *Repo) SetRemote(owner, name string) (string, string, error) {
	return "", "", nil
}
func (mockrepo *Repo) CreateRemote(name string) error {
	switch name {
	case "existingPkg":
		return errors.New("already exists")
	case "existingPkg-1105":
		return errors.New("already exists")
	case "existing-Pkg":
		return errors.New("already exists")
	case "existing-Pkg-0":
		return errors.New("already exists")
	}

	return nil
}

func (mockrepo *Repo) Exists(name string) (bool, error) {
	return false, nil
}

func (mockrepo *Repo) OwnerAndNameFromRemote() (string, string, error) {
	return "", "", nil
}

func (mockrepo *Repo) AssertNotTracked() error {
	return nil
}

func (mockrepo *Repo) WorkingDir() (string, error) {
	return "", nil
}
