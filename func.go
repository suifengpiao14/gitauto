package gitauto

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"golang.org/x/time/rate"
)

func LocalFilename(remoteFilename string) (localFilename string) {
	remoteUrl, repositoryFilename := splitRemoteUrlAndRepositoryFilename(remoteFilename)
	workDir := GetWorkDir(remoteUrl)
	localFilename = fmt.Sprintf("%s/%s", strings.TrimRight(workDir, "/"), strings.TrimLeft(repositoryFilename, "/"))
	return localFilename
}

//RepositoryFilename 从本地或者远程文件名获取仓库内文件名
func RepositoryFilename(remoteOrLocalFilename string) (repositoryFilename string) {
	if strings.HasPrefix(remoteOrLocalFilename, RobotWorkDir) {
		return getRepositoryFilenameByLocalFilename(remoteOrLocalFilename)
	}
	_, repositoryFilename = splitRemoteUrlAndRepositoryFilename(remoteOrLocalFilename)
	return repositoryFilename
}

func GetWorkDir(remoteFilename string) (path string) {
	remoteUrl, _ := splitRemoteUrlAndRepositoryFilename(remoteFilename)
	u, err := parseRemoteUrl(remoteUrl)
	repositoryPath := remoteUrl
	if err == nil {
		repositoryPath = relativeWorkDir(u)
	}
	repositoryPath = strings.TrimSuffix(repositoryPath, git.GitDirName)
	path = fmt.Sprintf("%s/%s", RobotWorkDir, repositoryPath)
	path = strings.TrimRight(path, "/")
	return path
}

//splitRemoteUrlAndRepositoryFilename 从远程文件路径中识别出远程仓库地址和仓库下文件名,如果没有.git 标记，则全部当成filename 返回（批量设置文件内容时，有用到这个特性）
func splitRemoteUrlAndRepositoryFilename(remoteFilename string) (remoteUrl string, filename string) {
	gitIndex := strings.Index(remoteFilename, git.GitDirName)
	if gitIndex < 0 {
		return "", remoteFilename
	}
	index := gitIndex + 4
	remoteUrl, filename = remoteFilename[:index], remoteFilename[index:]
	filename = strings.TrimLeft(filename, "/") //仓库内文件，开头不用"/"
	return remoteUrl, filename
}

// getHasAuthRemoteUrlFromRepositoryConfig 获取仓库远程地址和验证配置,验证配置不存在时,返回最后一条远程地址,验证器返回空
func getHasAuthRemoteUrlFromRepositoryConfig(cfg *config.Config) (auth transport.AuthMethod, u *url.URL) {
	for _, remote := range cfg.Remotes {
		for _, remoteAddress := range remote.URLs {
			var err error
			u, err = parseRemoteUrl(remoteAddress)
			if err != nil {
				continue
			}
			auth, ok := GetAuth(u.User.Username(), u.Hostname())
			if ok {
				return auth, u
			}
		}
	}
	return nil, u
}

var pullLimiter sync.Map

// allowPull  此刻是否容许pull
func allowPull(repositoryPath string) (allow bool) {
	if AllowPullPeriod == 0 {
		AllowPullPeriod = 5 * time.Minute
	}
	newRateLimiter := rate.NewLimiter(rate.Every(AllowPullPeriod), 1) //5分钟内最多更新1次
	actual, _ := pullLimiter.LoadOrStore(repositoryPath, newRateLimiter)
	rateLimiter := actual.(*rate.Limiter)
	allow = rateLimiter.Allow()
	return allow
}

// relativeWorkDir 获取相对工作目录
func relativeWorkDir(u *url.URL) (repositoryPath string) {
	repositoryPath = fmt.Sprintf("%s/%s", strings.Trim(u.Hostname(), "/"), strings.Trim(u.Path, "/"))
	return repositoryPath
}

func parseRemoteUrl(remoteUrl string) (u *url.URL, err error) {
	u, err = url.Parse(remoteUrl)
	if err == nil {
		return u, nil
	}
	u, err = detectSSH(remoteUrl)
	if err != nil {
		return u, err
	}
	return u, nil
}

// Note that we do not have an SSH-getter currently so this file serves
// only to hold the detectSSH helper that is used by other detectors.

// sshPattern matches SCP-like SSH patterns (user@host:path)
var sshPattern = regexp.MustCompile("^(?:([^@]+)@)?([^:]+):/?(.+)$")

// detectSSH determines if the src string matches an SSH-like URL and
// converts it into a net.URL compatible string. This returns nil if the
// string doesn't match the SSH pattern.
//
// This function is tested indirectly via detect_git_test.go
func detectSSH(src string) (*url.URL, error) {
	matched := sshPattern.FindStringSubmatch(src)
	if matched == nil {
		return nil, nil
	}

	user := matched[1]
	host := matched[2]
	path := matched[3]
	tmpU, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	var u url.URL
	u.Scheme = "ssh"
	u.User = url.User(user)
	u.Host = host
	u.Path = tmpU.Path
	u.RawQuery = tmpU.RawQuery
	u.Fragment = tmpU.Fragment

	return &u, nil
}

func getRepositoryFilenameByLocalFilename(localFilename string) (repositoryFilename string) {
	var split = `/`
	if strings.Contains(localFilename, `\`) {
		split = `\`
	}
	filename := strings.TrimRight(localFilename, split)
	for {
		lastSlashIndex := strings.LastIndex(filename, split)
		if lastSlashIndex > -1 {
			var basename string
			filename, basename = filename[:lastSlashIndex], filename[lastSlashIndex+1:]
			repositoryFilename = fmt.Sprintf("%s/%s", basename, repositoryFilename)
			gitDir := fmt.Sprintf("%s/%s", filename, git.GitDirName)
			if IsDir(gitDir) {
				break
			}
		} else {
			break
		}
	}
	repositoryFilename = strings.Trim(repositoryFilename, "/")
	return repositoryFilename
}

func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {

		return false
	}
	return s.IsDir()

}
