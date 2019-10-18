package main

import (
	"crypto/sha1"
	"encoding/base64"
	"flag"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type DependencyInfo struct {
	Name            string
	Version         string
	LicenseName     string
	LicenseFullName string
	LicenseHash     string
	LicensePath     string
	NoticePath      string
	NoticeHash      string
	NewNoticeName   string
	NewLicenseName  string
}

type LicenseHashInfo struct {
	LicenseName string
	LicenseHash string
}

type LicenseHashInfos []LicenseHashInfo

func (lhs LicenseHashInfos) Len() int {
	return len(lhs)
}

func (lhs LicenseHashInfos) Less(i, j int) bool {
	return  lhs[i].LicenseName < lhs[j].LicenseName
}

func (lhs LicenseHashInfos) Swap(i ,j int) {
	lhs[i], lhs[j] = lhs[j], lhs[i]

}

/*var LicenseRegexpArr = [...][2]string {
{"The Apache Software License, Version 2.0", `\bApache\b.+\b2`},
{"The Apache Software License, Version 1.1", `\bApache\b.+\b1.1\b`},
{"Apache Public License 2.0", `\bApache\b.+\bPublic\b.+\b2`},
{"BSD 4-Clause DOM4", `(\bBSD\b.+\b4)|\b4.+\bBSD\b`},
{"BSD 3-Clause", `(\bBSD\b.+\b3)|\b3\.+\bBSD\b`},
{"Eclipse Public License, Version 1.0", `\bEclipse\b.+\b1`},
{"Lesser General Public License (LGPL) v 3.0", `\b((Lesser General Public License)|LGPL)\b.+\b3`},
{"MIT License", "\bMIT\b"},
{"New BSD License", `\bNew\b.+\bBSD\b`},
}

*/

func main() {
	dependencyFile := flag.String("dependency", "", "dependency file name path, e.g -dependency=../servicecomb-kie/go.mod")
	licenseFile := flag.String("license", "", "LICENSE file name path, e.g -license=../servicecomb-kie/LICENSE, default: LICENSE in the dir of dependency file")
	noticeFile := flag.String("notice", "", "NOTICE file name path, e.g -dependency=../servicecomb-kie/LICENSE, default: NOTICE in the dir of dependency file")
	outputDir := flag.String("outputdir", "./dependency-licenses", "output directory path, e.g -outputdir=../dependency-licenses, default: ./dependency-licenses")
	projectName := flag.String("projectname", "", "projectname, e.g -projectname=.\"Apache servicecomb-mesher\", default: \"\"")
	flag.Parse()
	if *licenseFile == "" {
		*licenseFile = filepath.Dir(*dependencyFile) + "/" + "LICENSE"
	}
	if *noticeFile == "" {
		*noticeFile = filepath.Dir(*dependencyFile) + "/" + "NOTICE"
	}
	dependencyInfoArr := dependenciesList(*dependencyFile)
	downloadDependencies(*dependencyFile)
	analyseLicenses2(*dependencyFile, dependencyInfoArr)
	outputFile(*dependencyFile, dependencyInfoArr, *licenseFile,*noticeFile, *outputDir, *projectName)

}
func execCommand(cmdStr string, args ...string) (res string) {
	cmd := exec.Command(cmdStr, args...)
	if resByte, err := cmd.Output(); err != nil {
		fmt.Println(string(resByte), err)
		//os.Exit(1)
	} else {
		res = string(resByte[:])
	}
	return
}

func localRepPath(dependencyFile string) (res string) {
	res = execCommand("go", "env", "GOPATH")
	res = res[:len(res)-1] + "/pkg/mod"
	return
}

func downloadDependencies(dependencyFile string) {
	dir := filepath.Dir(dependencyFile)
	os.Chdir(dir)

	execCommand("go", "mod", "download")

}

func dependenciesList(dependencyFile string) (res []DependencyInfo) {
	dir := filepath.Dir(dependencyFile)
	os.Chdir(dir)
	dArrStr := ""
	res = make([]DependencyInfo, 0)

	dArrStr = execCommand("go", "list", "-m", "all")
	dArr := strings.Split(dArrStr, "\n")

	for _, d := range dArr {
		dTokenArr := strings.Split(d, " ")
		if len(dTokenArr) > 1 {
			res = append(res, DependencyInfo{Name: dTokenArr[0], Version: dTokenArr[1]})
		}
	}

	return
}


func analyseLicenses2(dependencyFile string, dArr []DependencyInfo) (res []DependencyInfo) {

	repPath := localRepPath(dependencyFile)
	os.Chdir(repPath)
	for i, d := range dArr {
		pathTmp := repPath + "/" + d.Name + "@" + d.Version
		if licenseJsonStr := execCommand("licensee", "detect", "--json", "--no-packages","--confidence=95", pathTmp); licenseJsonStr != "" {
			if size := jsoniter.Get([]byte(licenseJsonStr), "matched_files").Size(); size == 0 {
				fmt.Println("errors!!! cannot find license, the size of license is ", size, " for "+d.Name+"@"+d.Version)
				continue
			} else if size > 1 {
				lFileID := jsoniter.Get([]byte(licenseJsonStr), "matched_files", 0, "matched_license").ToString()
				lID := jsoniter.Get([]byte(licenseJsonStr), "licenses", 0, "spdx_id").ToString()
				if !(lFileID == lID) {
					fmt.Println("ERROR !!! There are more than 2 LICENSE file find!!")
				}
			}
			lFileName := jsoniter.Get([]byte(licenseJsonStr), "matched_files", 0, "filename").ToString()
			dArr[i].LicensePath = filepath.Join(repPath, d.Name+"@"+d.Version, lFileName)
			dArr[i].LicenseName = jsoniter.Get([]byte(licenseJsonStr), "matched_files", 0, "matched_license").ToString()
			dArr[i].LicenseFullName = jsoniter.Get([]byte(licenseJsonStr), "licenses", 0, "meta", "title").ToString()
			licenseBytes, _ := ioutil.ReadFile(dArr[i].LicensePath)
			hasher := sha1.New()
			hasher.Write(licenseBytes)
			sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
			dArr[i].LicenseHash = sha
		} else {
			fmt.Println("Find license error !!! cannot find license file for ", d.Name+"@"+d.Version)
		}

		if noticeName := execCommand("bash", "-c","ls -1 " + d.Name + "@"+ d.Version + " | grep  -i  NOTICE"); noticeName != "" {
			noticeName = noticeName[:len(noticeName) - 1]
			dArr[i].NoticePath = filepath.Join(repPath, d.Name + "@"+ d.Version, noticeName)
			noticeBytes, _ := ioutil.ReadFile(dArr[i].LicensePath)
			hasher := sha1.New()
			hasher.Write(noticeBytes)
			sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
			dArr[i].NoticeHash = sha
		}
	}

	return dArr
}

func outputFile(dependencyFile string, dArr []DependencyInfo, licenseFile string, noticeFile string, outputDir string, projectName string) {
	licenseMap := make(map[string][]*DependencyInfo)
	//dir := filepath.Dir(os.Args[0])
	//os.Chdir(dir)
	os.Chdir("/Users/zizipo/go_projects/dependency-license-collect")
	if err := os.Mkdir(outputDir, 0755); err != nil {
		fmt.Println(err)
	}

	selfLicenseBytes, _ := ioutil.ReadFile(licenseFile)
	selfLicenseStr := string(selfLicenseBytes)
	subComponentHeader := `
=======================================================================
%s Subcomponents:

The %s project contains subcomponents with
seperate copyright notices and license terms. Your use of the binary release
for these subcomponents is subject to the terms and conditions of the
following licenses.

`
	subComponentTitle := `
================================================================
The following component(s) are provided under the %s License (%s).
See the respective project link for details.
You can find a copy of the License at %s.

`
	subComponentLine := `%s (%s)`

	selfLicenseStr += fmt.Sprintf(subComponentHeader, projectName, projectName)
	for i, d := range dArr {
		//转置map
		if licenseMap[d.LicenseHash] == nil {
			licenseMap[d.LicenseHash] = make([]*DependencyInfo, 1)
			licenseMap[d.LicenseHash][0] = &dArr[i]
		} else {
			licenseMap[d.LicenseHash] = append(licenseMap[d.LicenseHash], &dArr[i])
		}
	}

	licenseHashInfos := make(LicenseHashInfos, 0)

	for k, v := range licenseMap {

		dNameTokens := strings.Split(v[0].Name, "/")
		licenseMap[k][0].NewLicenseName = filepath.Base(v[0].LicensePath) + "_" + dNameTokens[len(dNameTokens)-2] + "_" + dNameTokens[len(dNameTokens)-1]

		execCommand("cp", v[0].LicensePath, outputDir+"/" + licenseMap[k][0].NewLicenseName)
		licenseHashInfos = append(licenseHashInfos, LicenseHashInfo{LicenseName: v[0].LicenseName, LicenseHash: k})
	}
	sort.Sort(licenseHashInfos)


	for _, v := range licenseHashInfos {
		d := licenseMap[v.LicenseHash][0]
		selfLicenseStr += fmt.Sprintf(subComponentTitle, d.LicenseName, d.LicenseFullName, filepath.Base(d.NewLicenseName))
		for _, d2 := range licenseMap[v.LicenseHash] {
			selfLicenseStr += fmt.Sprintf(subComponentLine, d2.Name, d2.Version) + "\n"
		}

	}

	ioutil.WriteFile(outputDir+"/"+"LICENSE", []byte(selfLicenseStr), 0444)



	subComponentHeaderN := `
=======================================================================
%s Subcomponents:

`
	subComponentTitleN := `
================================================================
You can find a copy of the Notice at %s for the the following component(s)
See the respective project link for details.
`
	subComponentLineN := `%s (%s)`
	noticeMap := make(map[string][]*DependencyInfo)
	selfNoticeBytes, _ := ioutil.ReadFile(noticeFile)
	selfNoticeStr := string(selfNoticeBytes)
	//selfNoticeStr := ""

	selfNoticeStr += fmt.Sprintf(subComponentHeaderN, projectName)
	for i, d := range dArr {
		//转置map
		if d.NoticeHash != "" {
			if noticeMap[d.NoticeHash] == nil {
				noticeMap[d.NoticeHash] = make([]*DependencyInfo, 1)
				noticeMap[d.NoticeHash][0] = &dArr[i]
			} else {
				noticeMap[d.NoticeHash] = append(noticeMap[d.LicenseHash], &dArr[i])
			}
		}
	}


	for k, v := range noticeMap {

		dNameTokens := strings.Split(v[0].Name, "/")
		noticeMap[k][0].NewNoticeName = filepath.Base(v[0].NoticePath) + "_" + dNameTokens[len(dNameTokens)-2] + "_" + dNameTokens[len(dNameTokens)-1]

		if licenseMap[k][0].NewNoticeName != "" {
			execCommand("cp", v[0].NoticePath, outputDir+"/"+licenseMap[k][0].NewNoticeName)
		}



		selfNoticeStr += fmt.Sprintf(subComponentTitleN, v[0].NewNoticeName)
		for _, d := range noticeMap[k] {
			selfNoticeStr += fmt.Sprintf(subComponentLineN, d.Name, d.Version) + "\n"
		}
	}

	ioutil.WriteFile(outputDir+"/"+"NOTICE", []byte(selfNoticeStr), 0444)



}
