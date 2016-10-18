package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"

	engineapi "github.com/docker/engine-api/client"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

const (
	letters       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	shortenLength = 7
	CpuPeriodBase = 100000
)

func RandomString(n int) string {
	r := make([]byte, n)
	for i := 0; i < n; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		// 没那么惨吧
		if err != nil {
			continue
		}
		r[i] = letters[n.Int64()]
	}
	return string(r)
}

func TruncateID(id string) string {
	if len(id) > shortenLength {
		return id[:shortenLength]
	}
	return id
}

func Tail(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func GetGitRepoName(url string) (string, error) {
	if !strings.HasPrefix(url, "git@") || !strings.HasSuffix(url, ".git") {
		return "", fmt.Errorf("Bad git url format %q", url)
	}

	x := strings.SplitN(url, ":", 2)
	if len(x) != 2 {
		return "", fmt.Errorf("Bad git url format %q", url)
	}

	y := strings.SplitN(x[1], "/", 2)
	if len(y) != 2 {
		return "", fmt.Errorf("Bad git url format %q", url)
	}
	return strings.TrimSuffix(y[1], ".git"), nil
}

func GetVersion(image string) string {
	if !strings.Contains(image, ":") {
		return "unknown"
	}

	parts := strings.Split(image, ":")
	if len(parts) != 2 {
		return "unknown"
	}

	return parts[1]
}

// Bind a docker engine client to context
func ToDockerContext(client *engineapi.Client) context.Context {
	return context.WithValue(context.Background(), "engine", client)
}

// Get a docker engine client from a context
func FromDockerContext(ctx context.Context) (*engineapi.Client, bool) {
	client, ok := ctx.Value("engine").(*engineapi.Client)
	return client, ok
}

func SaveFile(content, path string, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

type NodeInfo struct {
	Name    string
	CorePer int
	Memory  int64
}

type ByCoreNum []NodeInfo

func (a ByCoreNum) Len() int           { return len(a) }
func (a ByCoreNum) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByCoreNum) Less(i, j int) bool { return a[i].CorePer < a[j].CorePer }

func AllocContainerPlan(nodeInfo ByCoreNum, quota int, memory int64, count int) (map[string]int, error) {
	result := make(map[string]int)
	N := nodeInfo.Len()
	flag := -1

	for i := 0; i < N; i++ {
		if nodeInfo[i].CorePer >= quota {
			flag = i
			break
		}
	}
	if flag == -1 {
		return result, fmt.Errorf("Cannot alloc a plan, not enough cpu quota")
	}

	// 计算是否有足够的内存满足需求
	bucket := []NodeInfo{}
	volume := 0
	volNum := []int{} //为了排序
	for i := flag; i < N; i++ {
		temp := int(nodeInfo[i].Memory / memory)
		if temp > 0 {
			volume += temp
			bucket = append(bucket, nodeInfo[i])
			volNum = append(volNum, temp)
		}
	}
	if volume < count {
		return result, fmt.Errorf("Cannot alloc a plan, not enough memory.")
	}

	sort.Ints(volNum)
	plan := allocAlgorithm(volNum, count)

	for i, num := range plan {
		key := bucket[i].Name
		result[key] = num
	}
	return result, nil
}

func allocAlgorithm(info []int, need int) map[int]int {
	// 实际上，这就是精确分配时候的那个分配算法
	// 情景是相同的：我们知道每台机能否分配多少容器
	// 要求我们尽可能平均地分配
	// 算法的正确性我们之前确认了
	// 所以我抄了过来
	result := make(map[int]int)
	nnode := len(info)

	var nodeToUse, more int
	for i := 0; i < nnode; i++ {
		nodeToUse = nnode - i
		ave := need / nodeToUse
		if ave > info[i] {
			ave = 1
		}
		for ; ave < info[i] && ave*nodeToUse < need; ave++ {
		}
		more = ave*nodeToUse - need
		for j := i; nodeToUse != 0; nodeToUse-- {
			if _, ok := result[j]; !ok {
				result[j] = ave
			} else {
				result[j] += ave
			}
			if more > 0 {
				n, _ := rand.Int(rand.Reader, big.NewInt(int64(nodeToUse)))
				more--
				result[j+int(n.Int64())]--
			} else if more < 0 {
				info[j] -= ave
			}
			j++
		}
		if more == 0 {
			break
		}
		need = -more
	}
	return result
}

func GetNodesInfo(cpumap map[string]types.CPUAndMem) ByCoreNum {
	result := ByCoreNum{}
	for node, cpuandmem := range cpumap {
		result = append(result, NodeInfo{node, len(cpuandmem.CpuMap) * CpuPeriodBase, cpuandmem.MemCap})
	}
	sort.Sort(result)
	return result
}

// copied from https://gist.github.com/jmervine/d88c75329f98e09f5c87
func safeSplit(s string) []string {
	split := strings.Split(s, " ")

	var result []string
	var inquote string
	var block string
	for _, i := range split {
		if inquote == "" {
			if strings.HasPrefix(i, "'") || strings.HasPrefix(i, "\"") {
				inquote = string(i[0])
				block = strings.TrimPrefix(i, inquote) + " "
			} else {
				result = append(result, i)
			}
		} else {
			if !strings.HasSuffix(i, inquote) {
				block += i + " "
			} else {
				block += strings.TrimSuffix(i, inquote)
				inquote = ""
				result = append(result, block)
				block = ""
			}
		}
	}

	return result
}

func MakeCommandLineArgs(s string) []string {
	r := []string{}
	for _, part := range safeSplit(s) {
		if len(part) == 0 {
			continue
		}
		r = append(r, part)
	}
	return r
}
