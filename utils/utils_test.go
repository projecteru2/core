package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomString(t *testing.T) {
	s1 := RandomString(10)
	assert.Equal(t, 10, len(s1))
	s2 := RandomString(10)
	assert.Equal(t, 10, len(s2))
	assert.NotEqual(t, s1, s2, fmt.Sprintf("s1: %s, s2: %s", s1, s2))
}

func TestTruncateID(t *testing.T) {
	r1 := TruncateID("1234")
	assert.Equal(t, r1, "1234")
	r2 := TruncateID("12345678")
	assert.Equal(t, r2, "1234567")
}

func TestTail(t *testing.T) {
	r1 := Tail("")
	assert.Equal(t, r1, "")
	r2 := Tail("/")
	assert.Equal(t, r2, "")
	r3 := Tail("a/b")
	assert.Equal(t, r3, "b")
	r4 := Tail("a/b/c")
	assert.Equal(t, r4, "c")
}

func TestGetGitRepoName(t *testing.T) {
	_, err := GetGitRepoName("xxx")
	assert.Error(t, err)

	_, err = GetGitRepoName("http://gitlab.ricebook.net/platform/core.git")
	assert.Error(t, err)

	r1, err := GetGitRepoName("git@gitlab.ricebook.net:platform/core.git")
	assert.NoError(t, err)
	assert.Equal(t, r1, "core")
}

func updateByCoreNum(input ByCoreNum, plan map[string]int, memory int64) ByCoreNum {
	result := ByCoreNum{}
	for _, node := range input {
		memoryChange := int64(plan[node.Name]) * memory
		node.Memory -= memoryChange
		result = append(result, node)
	}
	return result
}

func TestContinuousAddingContainer(t *testing.T) {
	// 测试分配算法随机性
	testPodInfo := ByCoreNum{}
	node1 := NodeInfo{"n1", 20000, 3 * 1024 * 1024 * 1024}
	node2 := NodeInfo{"n2", 30000, 3 * 1024 * 1024 * 1024}
	//	node3 := NodeInfo{"n3", 10000}
	testPodInfo = append(testPodInfo, node1)
	testPodInfo = append(testPodInfo, node2)
	//	testPodInfo = append(testPodInfo, node3)

	for i := 0; i < 7; i++ {
		res, err := AllocContainerPlan(testPodInfo, 10000, 512*1024*1024, 1)
		testPodInfo = updateByCoreNum(testPodInfo, res, 512*1024*1024)
		// fmt.Println(testPodInfo)
		fmt.Println(res)
		assert.NoError(t, err)
	}
}

func TestAlgorithm(t *testing.T) {
	testPodInfo := ByCoreNum{}
	node1 := NodeInfo{"n1", 30000, 5 * 512 * 1024 * 1024}
	node2 := NodeInfo{"n2", 30000, 5 * 512 * 1024 * 1024}
	node3 := NodeInfo{"n3", 30000, 5 * 512 * 1024 * 1024}
	node4 := NodeInfo{"n3", 30000, 5 * 512 * 1024 * 1024}
	testPodInfo = append(testPodInfo, node1)
	testPodInfo = append(testPodInfo, node2)
	testPodInfo = append(testPodInfo, node3)
	testPodInfo = append(testPodInfo, node4)

	res1, _ := AllocContainerPlan(testPodInfo, 10000, 512*1024*1024, 1)
	fmt.Println(res1)
}

func TestAllocContainerPlan(t *testing.T) {
	testPodInfo := ByCoreNum{}
	node1 := NodeInfo{"n1", 30000, 512 * 1024 * 1024}
	node2 := NodeInfo{"n2", 30000, 3 * 512 * 1024 * 1024}
	node3 := NodeInfo{"n3", 30000, 5 * 512 * 1024 * 1024}
	testPodInfo = append(testPodInfo, node1)
	testPodInfo = append(testPodInfo, node2)
	testPodInfo = append(testPodInfo, node3)

	res1, _ := AllocContainerPlan(testPodInfo, 10000, 512*1024*1024, 3)
	assert.Equal(t, res1["n1"], 1)
	assert.Equal(t, res1["n2"], 1)
	assert.Equal(t, res1["n3"], 1)

	res2, _ := AllocContainerPlan(testPodInfo, 10000, 512*1024*1024, 5)
	assert.Equal(t, res2["n1"], 1)
	assert.Equal(t, res2["n2"], 2)
	assert.Equal(t, res2["n3"], 2)

	res3, _ := AllocContainerPlan(testPodInfo, 10000, 512*1024*1024, 6)
	assert.Equal(t, res3["n1"], 1)
	assert.True(t, res3["n2"] >= 2)
	fmt.Printf("Container in n2: %d.\n", res3["n2"])
	assert.True(t, res3["n3"] >= 2)
	fmt.Printf("Container in n3: %d.\n", res3["n3"])

	_, err := AllocContainerPlan(testPodInfo, 10000, 512*1024*1024, 10)
	assert.Error(t, err, "Cannot alloc a plan, not enough memory.")

}

func TestDebugPlan(t *testing.T) {
	testPodInfo := ByCoreNum{}
	node0 := NodeInfo{"C2-docker-14", 1600000, 13732667392}
	node1 := NodeInfo{"C2-docker-15", 1600000, 4303872000}
	node2 := NodeInfo{"C2-docker-16", 1600000, 1317527552}
	node3 := NodeInfo{"C2-docker-17", 1600000, 8808554496}
	node4 := NodeInfo{"C2-docker-22", 1600000, 1527242752}
	node5 := NodeInfo{"C2-docker-23", 1600000, 9227984896}

	testPodInfo = append(testPodInfo, node0)
	testPodInfo = append(testPodInfo, node1)
	testPodInfo = append(testPodInfo, node2)
	testPodInfo = append(testPodInfo, node3)
	testPodInfo = append(testPodInfo, node4)
	testPodInfo = append(testPodInfo, node5)

	res, _ := AllocContainerPlan(testPodInfo, 100000, 2357198848, 10)
	fmt.Printf("result: %v\n", res)
	assert.True(t, res["C2-docker-15"] == 1)
}

func TestAllocAlgorithm(t *testing.T) {
	testPodInfo := ByCoreNum{}
	node0 := NodeInfo{"C2-docker-15", 1600000, 0}
	node1 := NodeInfo{"C2-docker-16", 1600000, 2357198848 * 2}
	node2 := NodeInfo{"C2-docker-17", 1600000, 2357198848 * 2}
	node3 := NodeInfo{"C2-docker-14", 1600000, 2357198848 * 2}
	testPodInfo = append(testPodInfo, node0)
	testPodInfo = append(testPodInfo, node1)
	testPodInfo = append(testPodInfo, node2)
	testPodInfo = append(testPodInfo, node3)

	res, _ := AllocContainerPlan(testPodInfo, 100000, 2357198848, 1)
	fmt.Printf("result: %v\n", res)
	assert.True(t, res["C2-docker-14"] == 1)

}

func TestMakeCommandLine(t *testing.T) {
	r1 := MakeCommandLineArgs("/bin/bash -l -c 'echo \"foo bar bah bin\"'")
	assert.Equal(t, r1, []string{"/bin/bash", "-l", "-c", "echo \"foo bar bah bin\""})
	r2 := MakeCommandLineArgs(" test -a   -b   -d")
	assert.Equal(t, r2, []string{"test", "-a", "-b", "-d"})
}

func TestMakeContainerName(t *testing.T) {
	r1 := MakeContainerName("test_appname", "web", "whatever")
	assert.Equal(t, r1, "test_appname_web_whatever")
	appname, entrypoint, ident, err := ParseContainerName("test_appname_web_whatever")
	assert.Equal(t, appname, "test_appname")
	assert.Equal(t, entrypoint, "web")
	assert.Equal(t, ident, "whatever")
	assert.Equal(t, err, nil)
}
