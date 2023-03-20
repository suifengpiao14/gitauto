package gitauto

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

var RobotWorkDir = "/tmp/dml"
var AllowPullPeriod time.Duration

type Repository struct {
	_auth       transport.AuthMethod
	_r          *git.Repository
	RemoteName  string
	LocalBranch string
}
type User struct {
	Name  string
	Email string
}

func NewRepository(remoteUrl string) (rc *Repository, err error) {
	if remoteUrl == "" {
		err = errors.Errorf("getRepository:remoteUrl not empty ")
		return nil, err
	}
	rc = &Repository{
		RemoteName: "origin",
	}
	workDir := GetWorkDir(remoteUrl)
	rc._r, err = git.PlainOpen(workDir)
	if errors.Is(err, git.ErrRepositoryNotExists) { // 仓库不存在,clone
		err = nil
		rc._r, err = clone(remoteUrl)
		if err != nil {
			return nil, err
		}
	}
	// 获取HEAD引用
	head, err := rc._r.Head()
	if err != nil {
		return nil, err
	}
	rc.LocalBranch = strings.TrimPrefix(head.Name().String(), "refs/heads/")
	cfg, err := rc._r.Config()
	if err != nil {
		return nil, err
	}
	auth, u := getHasAuthRemoteUrlFromRepositoryConfig(cfg)
	if auth != nil {
		rc._auth = auth
		return rc, nil
	}
	if u != nil {
		rc._auth, _ = GetAuth(u.User.Username(), u.Hostname())
	}
	return rc, nil
}

func (rc *Repository) ReadFile(filename string) (b []byte, err error) {
	w, err := rc._r.Worktree()
	if err != nil {
		return nil, err
	}
	f, err := w.Filesystem.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err = io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (rc *Repository) CreateBranch(branchName string) (err error) {
	r := rc._r
	localRef := plumbing.NewBranchReferenceName(branchName)
	err = r.CreateBranch(&config.Branch{
		Name:   branchName,
		Remote: rc.RemoteName,
		Merge:  localRef,
	})
	if errors.Is(err, git.ErrBranchExists) {
		err = nil
		return nil
	}
	if err != nil {
		return err
	}
	remoteRef := plumbing.NewRemoteReferenceName(rc.RemoteName, branchName)
	hashRef, err := r.Reference(remoteRef, true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		err = nil
		headRef, err := r.Head()
		if err != nil {
			return err
		}
		hashRef = plumbing.NewHashReference(remoteRef, headRef.Hash())
	}
	if err != nil {
		return err
	}
	newReference := plumbing.NewHashReference(localRef, hashRef.Hash())
	if err := r.Storer.SetReference(newReference); err != nil {
		return err
	}
	return nil
}

func (rc *Repository) Checkout() (err error) {
	w, err := rc._r.Worktree()
	if err != nil {
		return err
	}
	err = w.Checkout(&git.CheckoutOptions{
		Force: true,
	})

	if err != nil {
		return err
	}
	return
}

func (rc *Repository) Pull() (err error) {
	w, err := rc._r.Worktree()
	if err != nil {
		return err
	}
	err = w.Pull(&git.PullOptions{
		Auth:  rc._auth,
		Force: true,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) { //already up-to-date 为正常情况
		err = nil
	}
	if err != nil {
		return err
	}
	return
}

func (rc *Repository) CommitWithPush(commitMsg string, user User) (err error) {
	if user.Email == "" {
		err = errors.Errorf("user.Email not be empty")
		return err
	}
	r := rc._r
	if err != nil {
		return err
	}
	cfg, err := r.Config()
	if err != nil {
		return err
	}
	auth, u := getHasAuthRemoteUrlFromRepositoryConfig(cfg)
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	status, err := w.Status()
	if err != nil {
		return err
	}
	if status.IsClean() {
		return nil
	}

	addPath := "."
	_, err = w.Add(addPath)
	if err != nil {
		return err
	}

	_, err = w.Commit(commitMsg, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  user.Name,
			Email: user.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}
	err = w.Pull(&git.PullOptions{
		Auth:      auth,
		RemoteURL: u.String(),
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) { //already up-to-date 为正常情况
		err = nil
	}
	if errors.Is(err, git.ErrNonFastForwardUpdate) { //ErrNonFastForwardUpdate 为正常情况
		err = nil
	}
	if err != nil {
		return err
	}

	branchName := rc.LocalBranch
	refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branchName, branchName))

	err = r.Push(&git.PushOptions{
		Auth:      auth,
		RemoteURL: u.String(),
		RefSpecs: []config.RefSpec{
			refSpec,
		},
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		err = nil
	}
	if err != nil {
		return err
	}
	return nil
}

// AddReplaceFileToStage 新增、重置文件内容,并执行 git add .
func (rc *Repository) AddReplaceFileToStage(remoteFilename string, content []byte) (err error) {
	r := rc._r
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	filename := RepositoryFilename(remoteFilename)
	billyFile, err := w.Filesystem.OpenFile(filename, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		return err
	}
	defer billyFile.Close()
	err = billyFile.Truncate(0)
	if err != nil {
		return err
	}
	_, err = billyFile.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = billyFile.Write(content)
	if err != nil {
		return err
	}
	return nil
}

func (rc *Repository) DeleteFile(remoteFilenames ...string) (err error) {
	r := rc._r
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	for _, remoteFilename := range remoteFilenames {
		filename := RepositoryFilename(remoteFilename)
		err = w.Filesystem.Remove(filename)
		if err != nil {
			return err
		}
	}
	err = rc.AddAll()
	if err != nil {
		return err
	}
	return nil
}

func (rc *Repository) AddAll() (err error) {
	w, err := rc._r.Worktree()
	if err != nil {
		return err
	}
	_, err = w.Add(".")
	if err != nil {
		return err
	}
	return nil
}

func clone(remoteUrl string) (r *git.Repository, err error) {
	remoteUrl, _ = splitRemoteUrlAndRepositoryFilename(remoteUrl)
	remoteUrlObj, err := parseRemoteUrl(remoteUrl)
	if err != nil {
		return nil, err
	}
	auth, _ := GetAuth(remoteUrlObj.User.Username(), remoteUrlObj.Hostname())
	workDir := GetWorkDir(remoteUrl)
	cloneOptions := &git.CloneOptions{
		Auth: auth,
		URL:  remoteUrl,
	}

	r, err = git.PlainClone(workDir, false, cloneOptions)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// ReadFile 获取文件内容 path=ssh://git@gitea.programmerfamily.com:2221/go/coupon.git/doc/advertise/admin/adAdd.md,path=git@github.com:suifengpiao14/apidml/example/doc/addAdd.md
func ReadFile(remoteFilename string) (b []byte, err error) {
	remoteUrl, filename := splitRemoteUrlAndRepositoryFilename(remoteFilename)
	rc, err := NewRepository(remoteUrl)
	if err != nil {
		return nil, err
	}
	workDir := GetWorkDir(remoteUrl)
	if allowPull(workDir) {
		err = rc.Checkout()
		if err != nil {
			return nil, err
		}
		err = rc.Pull()
		if err != nil {
			return nil, err
		}
	}
	return rc.ReadFile(filename)
}

// GetLineCodeAuthor 获取文件每行作者
func (rc *Repository) GetLineCodeAuthor(remoteOrLocalFilename string) (lineCodeAuthors LineCodeAuthors, err error) {
	repositoryFileName := RepositoryFilename(remoteOrLocalFilename)
	r := rc._r
	lineCodeAuthors = make(LineCodeAuthors, 0)
	headRef, err := r.Head()
	if err != nil {
		return nil, err
	}
	headCommit, err := r.CommitObject(headRef.Hash())
	if err != nil {
		return nil, err
	}
	blameResult, err := git.Blame(headCommit, repositoryFileName)
	if err != nil {
		return nil, err
	}
	for i, line := range blameResult.Lines {
		lineAuthor := LineWithAuthor{
			LinNo:  i,
			Text:   line.Text,
			Author: Author(line.Author),
			Time:   line.Date,
		}
		lineCodeAuthors = append(lineCodeAuthors, lineAuthor)
	}
	return lineCodeAuthors, nil
}

func (rc *Repository) Exists(remoteOrLocalFilename string) (exits bool, err error) {
	repositoryFileName := RepositoryFilename(remoteOrLocalFilename)
	r := rc._r
	headRef, err := r.Head()
	if err != nil {
		return false, err
	}
	headCommit, err := r.CommitObject(headRef.Hash())
	if err != nil {
		return false, err
	}
	_, err = headCommit.File(repositoryFileName)
	if err != nil {
		if errors.Is(err, object.ErrFileNotFound) {
			return false, nil
		}
		return false, err
	}
	return false, nil
}

// 每行代码附带作者
type LineWithAuthor struct {
	LinNo  int
	Text   string
	Author Author
	Time   time.Time
}

type LineCodeAuthors []LineWithAuthor

func (lcas *LineCodeAuthors) Add(lineWithAuthors ...LineWithAuthor) {
	if lcas == nil {
		*lcas = make(LineCodeAuthors, 0)
	}
	*lcas = append(*lcas, lineWithAuthors...)
}

func (lcas LineCodeAuthors) GetOneLineAuthors(lineNo int) (lwca LineWithAuthor, ok bool) {
	if lineNo > len(lcas) {
		return lwca, false
	}
	lwca = lcas[lineNo-1]
	return lwca, true
}

// GetAuths 获取某段代码的作者
func (lcas LineCodeAuthors) GetMutilLineAuthors(star, end int) (authors Authors) {
	authors = make(Authors, 0)
	authorMap := make(map[Author]struct{})
	l := len(lcas)
	if star >= l {
		return authors
	}
	if star-1 < 0 {
		star = 1
	}

	for i := star - 1; i < end; i++ {
		if i >= l {
			break
		}
		author := Author(lcas[i].Author)
		authorMap[author] = struct{}{}
	}

	for auth := range authorMap {
		authors = append(authors, auth)
	}
	return authors
}

// CreateLineCodeAuthorsFromIOReader 根据文件内容,生成LineCodeAuthors
func CreateLineCodeAuthorsFromIOReader(reader io.Reader, author Author) (lcas LineCodeAuthors) {
	fileScanner := bufio.NewScanner(reader)
	fileScanner.Split(bufio.ScanLines)
	lcas = make(LineCodeAuthors, 0)
	i := 1
	for fileScanner.Scan() {
		lineWithAuthor := LineWithAuthor{
			LinNo:  i,
			Text:   fileScanner.Text(),
			Author: author,
			Time:   time.Now(),
		}
		lcas = append(lcas, lineWithAuthor)
	}
	return lcas
}

type Author string
type Authors []Author

func (a Authors) Len() int {
	return len(a)
}
func (a Authors) Has(author Author) (ok bool) {
	for _, author2 := range a {
		if author == author2 {
			return true
		}
	}
	return false
}
func (a Authors) Only(author Author) (ok bool) {
	ok = a.Len() == 1 && a.Has(author)
	return ok
}

func (a *Authors) AddIngore(authors ...Author) (ok bool) {
	for _, author := range authors {
		if a.Has(author) {
			continue
		}
		*a = append(*a, author)
	}
	return ok
}

func (a *Authors) Equal(authors Authors) (ok bool) {
	if len(*a) != len(authors) {
		return false
	}
	for _, author := range authors {
		if !a.Has(author) {
			return false
		}
	}
	return true
}
