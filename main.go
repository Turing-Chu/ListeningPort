// Copyright 2019 Turing Zhu
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Author: Turing Zhu
// Date: 2019-06-02 14:52
// File : main.go

package main

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/shirou/gopsutil/process"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	configFile := kingpin.Flag("config", "MySQL configuration file name").Short('c').Default("config.yml").String()
	kingpin.Version("listening_port")
	kingpin.CommandLine.GetFlag("help").Short('h')
	kingpin.Parse()

	processInfos, err := GetProcessInfos()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	err = store2DB(processInfos, configFile)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(2)
	}
}

type ProcessInfo struct {
	Port        int32     `json:"port"`
	Address     string    `json:"address"`
	Pid         int32     `json:"pid"`
	ProcessName string    `json:"process_name"`
	RootDir     string    `json:"root_dir"`
	User        string    `json:"user"`
	Type        string    `json:"_type" gorm:"column:_type"`
	Uptime      time.Time `json:"uptime"`
}

func GetProcessInfos() (processInfos []*ProcessInfo, err error) {
	processInfos, err = getNetworks()
	if err != nil {
		return []*ProcessInfo{}, err
	}
	for _, proc := range processInfos {
		err = proc.updateProcessInfo()
		if err != nil {
			_ = fmt.Errorf("%s", err.Error())
		}
	}
	return processInfos, nil
}

// 获取监听端口进程的 监听地址 监听类型 端口号 进程PID 进程名等信息
func getNetworks() (processInfos []*ProcessInfo, err error) {
	cmd := exec.Command("/usr/bin/netstat", "-ltp", "--numeric-ports", "--numeric-hosts")
	outputByte, err := cmd.Output()
	if err != nil {

		return nil, errors.New(fmt.Sprintf("parse output failed: %s", err.Error()))
	}

	outputs := strings.Split(string(outputByte), "\n")
	// tcp    0    0 0.0.0.0:6379    0.0.0.0:*    LISTEN    15238/redis-server
	match, err := regexp.Compile(`^(tcp\d?)\s+0\s+0\s+(\S+):(\d+)\s+\S+\s+LISTEN\s+(\d+)/(\S+)`)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s, %s", err.Error(), "正则编译失败"))
	}
	for _, line := range outputs {
		// 0:raw, 1:type, 2:address, 3:port, 4:pid, 5:process_name
		params := match.FindStringSubmatch(line)
		if len(params) == 0 {
			continue
		}
		port, _ := strconv.ParseInt(params[3], 10, 32)
		pid, _ := strconv.ParseInt(params[4], 10, 32)

		var addrType = "IPV4"
		if params[1] != "tcp" {
			addrType = "IPV6"
		}

		processInfos = append(processInfos, &ProcessInfo{
			Port:        int32(port),
			Address:     params[2],
			Pid:         int32(pid),
			Type:        addrType,
			ProcessName: params[5],
		})
	}
	return processInfos, err
}

// 完善进程信息
func (processInfo *ProcessInfo) updateProcessInfo() error {
	if processInfo.Pid == 0 {
		return errors.New(fmt.Sprintf("invalid ProcessInfo: %+v, Pid should not be empty", processInfo))
	}
	proc, _ := process.NewProcess(processInfo.Pid)

	username, err := proc.Username()
	if err != nil {
		return errors.New(fmt.Sprintf("get username failed, msg=%s", err.Error()))
	}
	processInfo.User = username

	rootDir, err := proc.Cwd()
	if err != nil {
		return errors.New(fmt.Sprintf("get current workspace directory failed, msg=%s", err.Error()))
	}
	processInfo.RootDir = rootDir

	timestamp, err := proc.CreateTime()
	if err != nil {
		return errors.New(fmt.Sprintf("get uptime failed, msg=%s", err.Error()))
	}
	processInfo.Uptime = time.Unix(timestamp/1000, 0)

	processName, err := proc.Name()
	if err != nil {
		return errors.New(fmt.Sprintf("get process name failed, msg=%s", err.Error()))
	}
	processInfo.ProcessName = processName

	return nil
}

func store2DB(processInfos []*ProcessInfo, cfg *string) error {
	config, err := getMysqlConfig(*cfg)
	if err != nil {
		return err
	}
	parameter := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		config.UserName, config.Password, config.Host, config.Port, config.DBName)
	db, err := gorm.Open("mysql", parameter)
	db = db.Table(config.TableName)
	defer func() {
		if db != nil {
			_ = db.Close()
		}
	}()
	if err != nil {
		fmt.Printf("connect database failed, msg=%s", err.Error())
		return fmt.Errorf("connect database failed, msg=%s", err.Error())
	}
	for _, proc := range processInfos {
		var tmpProcess ProcessInfo
		db.Table(config.TableName).First(&tmpProcess, "port = ?", proc.Pid)
		if db.RecordNotFound() {
			db.Create(&proc)
			continue
		}
		err = db.Where("port = ?", proc.Port).Update(&ProcessInfo{
			Address:     proc.Address,
			Type:        proc.Type,
			Pid:         proc.Pid,
			ProcessName: proc.ProcessName,
			User:        proc.User,
			RootDir:     proc.RootDir,
			Uptime:      proc.Uptime,
		}).Error
		if err != nil {
			fmt.Printf("update failed: %s", err.Error())
		}
	}
	return nil
}

type MysqlConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	UserName  string `yaml:"username"`
	Password  string `yaml:"password"`
	DBName    string `yaml:"db_name"`
	TableName string `yaml:"tb_name"`
}

func getMysqlConfig(filename string) (mysqlConifg MysqlConfig, err error) {
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return MysqlConfig{}, fmt.Errorf("yamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(yamlFile, &mysqlConifg)
	if err != nil {
		return MysqlConfig{}, fmt.Errorf("parse config failed, msg=%s", err.Error())
	}
	return mysqlConifg, nil
}
