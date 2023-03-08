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
	//privateKeyFile := `C:\Users\Admin\.ssh\id_rsa`
	privateKeyFile := `/Users/admin/.ssh/id_rsa`
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
	_, err := Clone(address)
	require.NoError(t, err)
}
func TestGitCloneGitHub(t *testing.T) {
	registerAuth()
	address := "git@github.com:suifengpiao14/apidml.git"
	_, err := Clone(address)
	require.NoError(t, err)
}

func TestGitOpen(t *testing.T) {
	registerAuth()
	LocalBranch = "dev"
	path := "ssh://git@gitea.programmerfamily.com:2221/go/coupon.git"
	_, err := GetRepository(path)
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

func TestGitSetFile(t *testing.T) {
	registerAuth()
	docName := "hello.md"
	path := fmt.Sprintf("ssh://git@gitea.programmerfamily.com:2221/go/coupon.git/doc/advertise/admin/doc/%s", docName)
	t.Run("create", func(t *testing.T) {
		b := []byte("hello world")
		err := SetFile(path, b)
		require.NoError(t, err)
	})

	t.Run("update", func(t *testing.T) {
		b := []byte("rewrite\n #hello world")
		err := SetFile(path, b)
		require.NoError(t, err)
	})

}

func TestGitPush(t *testing.T) {
	registerAuth()
	t.Run("push dev", func(t *testing.T) {
		LocalBranch = "dev"
		path := "ssh://git@gitea.programmerfamily.com:2221/go/coupon.git"
		err := Push(path, "push to dev")
		require.NoError(t, err)
	})

}
func TestGitDelete(t *testing.T) {
	registerAuth()
	docName := "hello.md"
	path := fmt.Sprintf("ssh://git@gitea.programmerfamily.com:2221/go/coupon.git/doc/advertise/admin/doc/%s", docName)
	err := DeleteFile(path)
	require.NoError(t, err)
}
