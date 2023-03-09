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
var LocalBranch = "master"
var RemoteName = "origin"

func GetRepository(remoteUrl string) (r *git.Repository, err error) {
	if remoteUrl == "" {
		err = errors.Errorf("getRepository:remoteUrl not empty ")
		return nil, err
	}
	workDir := getWorkDir(remoteUrl)
	r, err = git.PlainOpen(workDir)
	if errors.Is(err, git.ErrRepositoryNotExists) { // 仓库不存在,clone
		err = nil
		r, err = Clone(remoteUrl)
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	if err != nil {
		return nil, err
	}
	cfg, err := r.Config()
	if err != nil {
		return nil, err
	}
	var auth transport.AuthMethod
	auth, u := getHasAuthRemoteUrlFromRepositoryConfig(cfg)
	if u != nil {
		repositoryPath := relativeWorkDir(u)
		if !allowPull(repositoryPath) {
			return r, nil
		}
	}
	// 更新
	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}
	err = CreateBranch(remoteUrl, LocalBranch)
	if err != nil {
		return nil, err
	}
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(LocalBranch),
		Force:  true,
	})

	if err != nil {
		return nil, err
	}
	err = w.Pull(&git.PullOptions{
		Auth:  auth,
		Force: true,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) { //already up-to-date 为正常情况
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// ReadFile 获取文件内容 path=ssh://git@gitea.programmerfamily.com:2221/go/coupon.git/doc/advertise/admin/adAdd.md,path=git@github.com:suifengpiao14/apidml/example/doc/addAdd.md
func ReadFile(remoteFilename string) (b []byte, err error) {
	remoteUrl, filename := splitRemoteUrlAndFilename(remoteFilename)
	r, err := GetRepository(remoteUrl)
	if err != nil {
		return nil, err
	}
	w, err := r.Worktree()
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

func CreateBranch(remoteUrl string, branchName string) (err error) {
	r, err := GetRepository(remoteUrl)
	if err != nil {
		return err
	}
	localRef := plumbing.NewBranchReferenceName(branchName)
	err = r.CreateBranch(&config.Branch{
		Name:   LocalBranch,
		Remote: RemoteName,
		Merge:  localRef,
	})
	if errors.Is(err, git.ErrBranchExists) {
		err = nil
	}
	if err != nil {
		return err
	}
	remoteRef := plumbing.NewRemoteReferenceName(RemoteName, branchName)
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

func Clone(remoteUrl string) (r *git.Repository, err error) {
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

func Push(remoteUrl string, commitMsg string) (err error) {
	remoteUrl, _ = splitRemoteUrlAndFilename(remoteUrl)
	r, err := GetRepository(remoteUrl)
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

// AddReplaceFile 新增、重置文件内容
func AddReplaceFile(remoteFilename string, content []byte) (err error) {
	remoteUrl, filename := splitRemoteUrlAndFilename(remoteFilename)
	r, err := GetRepository(remoteUrl)
	if err != nil {
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}
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
	_, err = w.Add(filename)
	if err != nil {
		return err
	}
	return nil
}

func DeleteFile(remoteFilenames ...string) (err error) {
	if len(remoteFilenames) < 1 {
		return nil
	}
	firstRemoteFilename := remoteFilenames[0]
	remoteUrl, _ := splitRemoteUrlAndFilename(firstRemoteFilename)
	r, err := GetRepository(remoteUrl)
	if err != nil {
		return err
	}
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
	return nil
}
