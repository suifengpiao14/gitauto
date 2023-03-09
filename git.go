package gitauto

import (
	"io"
	"io/fs"
	"os"
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
var RobotName = "robot_apidml"
var RobotEmail = ""

type repository struct {
	_auth      transport.AuthMethod
	_r         *git.Repository
	RemoteName string
	User       User
}
type User struct {
	Name  string
	Email string
}

func NewRepository(remoteUrl string) (rc *repository, err error) {
	if remoteUrl == "" {
		err = errors.Errorf("getRepository:remoteUrl not empty ")
		return nil, err
	}
	rc = &repository{
		RemoteName: "origin",
		User: User{
			Name: "robot",
		},
	}
	workDir := getWorkDir(remoteUrl)
	rc._r, err = git.PlainOpen(workDir)
	if errors.Is(err, git.ErrRepositoryNotExists) { // 仓库不存在,clone
		err = nil
		rc._r, err = clone(remoteUrl)
		if err != nil {
			return nil, err
		}
	}
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

func (rc *repository) ReadFile(filename string) (b []byte, err error) {
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

func (rc *repository) CreateBranch(branchName string) (err error) {
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

func (rc *repository) Checkout() (err error) {
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

func (rc *repository) Pull() (err error) {
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

func (rc *repository) CommitWithPush(commitMsg string) (err error) {
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
	_, err = w.Add(".")
	if err != nil {
		return err
	}

	_, err = w.Commit(commitMsg, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  RobotName,
			Email: RobotEmail,
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
	if err != nil {
		return err
	}
	err = r.Push(&git.PushOptions{
		Auth:      auth,
		RemoteURL: u.String(),
	})
	if err != nil {
		return err
	}
	return nil
}

// AddReplaceFileToStage 新增、重置文件内容,并执行 git add .
func (rc *repository) AddReplaceFileToStage(remoteFilename string, content []byte) (err error) {
	r := rc._r
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	_, filename := splitRemoteUrlAndFilename(remoteFilename)
	billyFile, err := w.Filesystem.OpenFile(filename, os.O_RDWR, os.ModePerm)
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
		billyFile, err = w.Filesystem.Create(filename)
	}
	if err != nil {
		return err
	}
	defer billyFile.Close()
	_, err = billyFile.Write(content)
	if err != nil {
		return err
	}
	err = rc.AddAll()
	if err != nil {
		return err
	}
	return nil
}

func (rc *repository) DeleteFile(remoteFilenames ...string) (err error) {
	r := rc._r
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	for _, remoteFilename := range remoteFilenames {
		_, filename := splitRemoteUrlAndFilename(remoteFilename)
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

func (rc *repository) AddAll() (err error) {
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
	remoteUrlObj, err := parseRemoteUrl(remoteUrl)
	if err != nil {
		return nil, err
	}
	auth, _ := GetAuth(remoteUrlObj.User.Username(), remoteUrlObj.Hostname())
	workDir := getWorkDir(remoteUrl)
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
	remoteUrl, filename := splitRemoteUrlAndFilename(remoteFilename)
	rc, err := NewRepository(remoteUrl)
	if err != nil {
		return nil, err
	}
	workDir := getWorkDir(remoteUrl)
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
