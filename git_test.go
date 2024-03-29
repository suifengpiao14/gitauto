package gitauto

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registerAuth() {
	privateKeyFile := `C:\Users\Admin\.ssh\id_rsa`
	//privateKeyFile := `/Users/admin/.ssh/id_rsa`
	auth, err := ssh.NewPublicKeysFromFile("git", privateKeyFile, "")
	if err != nil {
		panic(err)
	}
	username := "git"
	host := "gitea.programmerfamily.com"
	hostGithub := "github.com"
	RegisterAuth(username, host, auth)
	RegisterAuth(username, hostGithub, auth)
}

func TestRead(t *testing.T) {
	registerAuth()
	t.Run("githubFormat", func(t *testing.T) {
		address := "git@github.com:suifengpiao14/apidml.git/example/doc/adList.md"
		b, err := ReadFile(address)
		require.NoError(t, err)
		s := string(b)
		fmt.Println(s)
	})

	t.Run("gitSSHFormat", func(t *testing.T) {
		address := "ssh://git@github.com/suifengpiao14/apidml.git/example/doc/adList.md"
		b, err := ReadFile(address)
		require.NoError(t, err)
		s := string(b)
		fmt.Println(s)
	})
}

func TestGitClone(t *testing.T) {
	registerAuth()
	address := "ssh://git@gitea.programmerfamily.com:2221/go/named.git"
	_, err := clone(address)
	require.NoError(t, err)
}
func TestGitCloneGitHub(t *testing.T) {
	registerAuth()
	address := "git@github.com:suifengpiao14/apidml.git"
	_, err := clone(address)
	require.NoError(t, err)
}

func TestGitOpen(t *testing.T) {
	registerAuth()
	path := "ssh://git@gitea.programmerfamily.com:2221/go/coupon.git"
	_, err := NewRepository(path)
	require.NoError(t, err)
}

func TestAllowPull(t *testing.T) {
	repositoryPath := "test"
	AllowPullPeriod = 2 * time.Minute

	allow := "allow"
	reject := "reject"
	actul := map[string]int{
		allow:  0,
		reject: 0,
	}
	var w sync.WaitGroup
	count := 6
	for i := 0; i < count; i++ {
		w.Add(1)
		if allowPull(repositoryPath) {
			actul[allow]++
			w.Done()
			continue
		}
		go func(i int) {
			time.Sleep(time.Duration(i) * time.Second)
			if allowPull(repositoryPath) {
				actul[allow]++
				w.Done()
				return
			}
			actul[reject]++
			w.Done()
		}(i)
	}
	w.Wait()
	expected := map[string]int{
		allow:  1,
		reject: count - 1,
	}
	assert.EqualValues(t, expected, actul)
}

func TestAddReplaceFileToStage(t *testing.T) {
	registerAuth()
	docName := "hello11.md"
	path := fmt.Sprintf("ssh://git@gitea.programmerfamily.com:2221/go/coupon.git/doc/advertise/admin/doc/%s", docName)
	rc, err := NewRepository(path)
	require.NoError(t, err)

	t.Run("create", func(t *testing.T) {
		b := []byte("hello world")
		err := rc.AddReplaceFileToStage(path, b)
		require.NoError(t, err)
	})

	t.Run("update", func(t *testing.T) {
		b := []byte("rewrite\n #hello world")
		err := rc.AddReplaceFileToStage(path, b)
		require.NoError(t, err)
	})

}

func TestCommitWithPush(t *testing.T) {
	registerAuth()
	path := "ssh://git@gitea.programmerfamily.com:2221/go/coupon.git"
	rc, err := NewRepository(path)
	require.NoError(t, err)
	t.Run("push dev", func(t *testing.T) {
		err := rc.CommitWithPush("push to dev", User{Name: "test"})
		require.NoError(t, err)
	})

}
func TestGitDelete(t *testing.T) {
	registerAuth()
	docName := "hello.md"
	path := fmt.Sprintf("ssh://git@gitea.programmerfamily.com:2221/go/coupon.git/doc/advertise/admin/doc/%s", docName)
	rc, err := NewRepository(path)
	require.NoError(t, err)
	err = rc.DeleteFile(path)
	require.NoError(t, err)
}

func TestGetFileLineAuthor(t *testing.T) {
	registerAuth()
	//docName := "admin_v1_ad_list.go"
	docName := "router.go"
	path := fmt.Sprintf("ssh://git@gitea.programmerfamily.com:2221/go/coupon.git/router/%s", docName)
	rc, err := NewRepository(path)
	require.NoError(t, err)
	lineAuths, err := rc.GetLineCodeAuthor(path)
	require.NoError(t, err)
	fmt.Println(lineAuths)
}

func TestGetFileLineAuthor2(t *testing.T) {
	registerAuth()
	//docName := "admin_v1_ad_list.go"
	docName := "git.go"
	path := fmt.Sprintf("git@github.com:suifengpiao14/gitauto.git/%s", docName)
	rc, err := NewRepository(path)
	require.NoError(t, err)
	lineAuths, err := rc.GetLineCodeAuthor(path)
	require.NoError(t, err)
	fmt.Println(lineAuths)
}

func TestGetRepositoryFilenameByLocalFilename(t *testing.T) {
	localFilename := `D:\tmp\dml\gitea.programmerfamily.com\go\coupon\router\api\ad\admin_v1_ad_list.go`
	repositoryFilename := getRepositoryFilenameByLocalFilename(localFilename)
	fmt.Println(repositoryFilename)
}
