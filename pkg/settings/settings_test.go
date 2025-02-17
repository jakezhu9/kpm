package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"kcl-lang.io/kpm/pkg/env"
	"kcl-lang.io/kpm/pkg/reporter"
	"kcl-lang.io/kpm/pkg/utils"
)

const testDataDir = "test_data"

func getTestDir(subDir string) string {
	pwd, _ := os.Getwd()
	testDir := filepath.Join(pwd, testDataDir)
	testDir = filepath.Join(testDir, subDir)

	return testDir
}

func TestSettingInit(t *testing.T) {
	kpmHome, err := env.GetAbsPkgPath()
	assert.Equal(t, err, nil)
	settings := GetSettings()
	assert.Equal(t, settings.ErrorEvent, (*reporter.KpmEvent)(nil))
	assert.Equal(t, settings.CredentialsFile, filepath.Join(kpmHome, CONFIG_JSON_PATH))
}

func TestGetFullJsonPath(t *testing.T) {
	path, err := GetFullPath("test.json")
	assert.Equal(t, err, nil)

	kpmHome, err := env.GetAbsPkgPath()
	assert.Equal(t, err, nil)
	assert.Equal(t, path, filepath.Join(kpmHome, "test.json"))
}

func TestDefaultKpmConf(t *testing.T) {
	settings := Settings{
		Conf: DefaultKpmConf(),
	}
	assert.Equal(t, settings.DefaultOciRegistry(), "ghcr.io")
	assert.Equal(t, settings.DefaultOciRepo(), "kcl-lang")
}

func TestLoadOrCreateDefaultKpmJson(t *testing.T) {
	testDir := getTestDir("expected.json")
	kpmPath := filepath.Join(filepath.Join(filepath.Join(filepath.Dir(testDir), ".kpm"), "config"), "kpm.json")
	err := os.Setenv("KCL_PKG_PATH", filepath.Dir(testDir))

	assert.Equal(t, err, nil)
	assert.Equal(t, utils.DirExists(kpmPath), false)

	kpmConf, err := loadOrCreateDefaultKpmJson()
	assert.Equal(t, kpmConf.DefaultOciRegistry, "ghcr.io")
	assert.Equal(t, kpmConf.DefaultOciRepo, "kcl-lang")
	assert.Equal(t, err, nil)
	assert.Equal(t, utils.DirExists(kpmPath), true)

	expectedJson, err := os.ReadFile(testDir)
	assert.Equal(t, err, nil)

	gotJson, err := os.ReadFile(kpmPath)
	assert.Equal(t, err, nil)

	var expected interface{}
	err = json.Unmarshal(expectedJson, &expected)
	assert.Equal(t, err, nil)

	var got interface{}
	err = json.Unmarshal(gotJson, &got)
	assert.Equal(t, err, nil)
	fmt.Printf("got: %v\n", got)
	fmt.Printf("expected: %v\n", expected)
	assert.Equal(t, reflect.DeepEqual(expected, got), true)

	os.RemoveAll(kpmPath)
	assert.Equal(t, utils.DirExists(kpmPath), false)
}

func TestPackageCacheLock(t *testing.T) {

	settings := GetSettings()
	assert.Equal(t, settings.ErrorEvent, (*reporter.KpmEvent)(nil))

	// create the expected result of the test.
	// 10 times of "goroutine 1: %d" at first, and then 10 times of "goroutine 2: %d"

	// If goroutine 1 get the lock first, then it will append "goroutine 1: %d" to the list.
	goroutine_1_first_list := []string{}

	for i := 0; i < 10; i++ {
		goroutine_1_first_list = append(goroutine_1_first_list, fmt.Sprintf("goroutine 1: %d", i))
	}

	for i := 0; i < 10; i++ {
		goroutine_1_first_list = append(goroutine_1_first_list, fmt.Sprintf("goroutine 2: %d", i))
	}

	// If goroutine 2 get the lock first, then it will append "goroutine 2: %d" to the list.
	goroutine_2_first_list := []string{}

	for i := 0; i < 10; i++ {
		goroutine_2_first_list = append(goroutine_2_first_list, fmt.Sprintf("goroutine 2: %d", i))
	}

	for i := 0; i < 10; i++ {
		goroutine_2_first_list = append(goroutine_2_first_list, fmt.Sprintf("goroutine 1: %d", i))
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// create a list to store the result generated by 2 goroutine concurrently.
	gotlist := []string{}

	// goroutine 1: append "goroutine 1: %d" to the list
	go func() {
		defer wg.Done()
		err := settings.AcquirePackageCacheLock()
		fmt.Printf("1: locked.")
		fmt.Printf("err: %v\n", err)
		for i := 0; i < 10; i++ {
			gotlist = append(gotlist, fmt.Sprintf("goroutine 1: %d", i))
		}
		err = settings.ReleasePackageCacheLock()
		fmt.Printf("err: %v\n", err)
		fmt.Printf("1: released.")
	}()

	// goroutine 2: append "goroutine 2: %d" to the list
	go func() {
		defer wg.Done()
		err := settings.AcquirePackageCacheLock()
		fmt.Printf("2: locked.")
		fmt.Printf("err: %v\n", err)
		for i := 0; i < 10; i++ {
			gotlist = append(gotlist, fmt.Sprintf("goroutine 2: %d", i))
		}
		err = settings.ReleasePackageCacheLock()
		fmt.Printf("err: %v\n", err)
		fmt.Printf("2: released.")
	}()

	wg.Wait()

	// Compare the gotlist and expectedlist.
	assert.Equal(t,
		(reflect.DeepEqual(gotlist, goroutine_1_first_list) || reflect.DeepEqual(gotlist, goroutine_2_first_list)),
		true)
}

func TestSettingEnv(t *testing.T) {
	settings := GetSettings()
	assert.Equal(t, settings.DefaultOciRegistry(), "ghcr.io")
	assert.Equal(t, settings.DefaultOciRepo(), "kcl-lang")
	assert.Equal(t, settings.DefaultOciPlainHttp(), false)

	err := os.Setenv("KPM_REG", "test_reg")
	assert.Equal(t, err, nil)
	err = os.Setenv("KPM_REPO", "test_repo")
	assert.Equal(t, err, nil)
	err = os.Setenv("OCI_REG_PLAIN_HTTP", "true")
	assert.Equal(t, err, nil)

	settings = GetSettings()
	assert.Equal(t, settings.DefaultOciRegistry(), "test_reg")
	assert.Equal(t, settings.DefaultOciRepo(), "test_repo")
	assert.Equal(t, settings.ErrorEvent.Type(), reporter.UnknownEnv)
	assert.Equal(t, settings.ErrorEvent.Error(), "kpm: unknown environment variable 'OCI_REG_PLAIN_HTTP=true'\nkpm: invalid environment variable.\n\n")
	assert.Equal(t, settings.DefaultOciPlainHttp(), false)

	err = os.Setenv("OCI_REG_PLAIN_HTTP", "on")
	assert.Equal(t, err, nil)
	settings = GetSettings()
	assert.Equal(t, settings.DefaultOciPlainHttp(), true)

	err = os.Setenv("OCI_REG_PLAIN_HTTP", "off")
	assert.Equal(t, err, nil)
	settings = GetSettings()
	assert.Equal(t, settings.DefaultOciPlainHttp(), false)
}
