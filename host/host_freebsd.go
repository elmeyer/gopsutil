// SPDX-License-Identifier: BSD-3-Clause
//go:build freebsd

package host

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/shirou/gopsutil/v4/internal/common"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	UTNameSize = 16 /* see MAXLOGNAME in <sys/param.h> */
	UTLineSize = 8
	UTHostSize = 16
)

func HostIDWithContext(_ context.Context) (string, error) {
	uuid, err := unix.Sysctl("kern.hostuuid")
	if err != nil {
		return "", err
	}
	return strings.ToLower(uuid), err
}

func numProcs(ctx context.Context) (uint64, error) {
	procs, err := process.PidsWithContext(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(len(procs)), nil
}

func UsersWithContext(_ context.Context) ([]UserStat, error) {
	utmpfile := "/var/run/utx.active"
	if !common.PathExists(utmpfile) {
		utmpfile = "/var/run/utmp" // before 9.0
		return getUsersFromUtmp(utmpfile)
	}

	var ret []UserStat
	file, err := os.Open(utmpfile)
	if err != nil {
		return ret, err
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		return ret, err
	}

	entrySize := sizeOfUtmpx
	count := len(buf) / entrySize

	for i := 0; i < count; i++ {
		b := buf[i*sizeOfUtmpx : (i+1)*sizeOfUtmpx]
		var u Utmpx
		br := bytes.NewReader(b)
		err := binary.Read(br, binary.BigEndian, &u)
		if err != nil || u.Type != 4 {
			continue
		}
		sec := math.Floor(float64(u.Tv) / 1000000)
		user := UserStat{
			User:     common.IntToString(u.User[:]),
			Terminal: common.IntToString(u.Line[:]),
			Host:     common.IntToString(u.Host[:]),
			Started:  int(sec),
		}

		ret = append(ret, user)
	}

	return ret, nil
}

func PlatformInformationWithContext(_ context.Context) (string, string, string, error) {
	platform, err := unix.Sysctl("kern.ostype")
	if err != nil {
		return "", "", "", err
	}

	version, err := unix.Sysctl("kern.osrelease")
	if err != nil {
		return "", "", "", err
	}

	return strings.ToLower(platform), "", strings.ToLower(version), nil
}

func VirtualizationWithContext(_ context.Context) (string, string, error) {
	return "", "", common.ErrNotImplementedError
}

// before 9.0
func getUsersFromUtmp(utmpfile string) ([]UserStat, error) {
	var ret []UserStat
	file, err := os.Open(utmpfile)
	if err != nil {
		return ret, err
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		return ret, err
	}

	entrySize := int(unsafe.Sizeof(Utmp{}))
	count := len(buf) / entrySize

	for i := 0; i < count; i++ {
		b := buf[i*entrySize : i*entrySize+entrySize]
		var u Utmp
		br := bytes.NewReader(b)
		err := binary.Read(br, binary.LittleEndian, &u)
		if err != nil || u.Time == 0 {
			continue
		}
		user := UserStat{
			User:     common.IntToString(u.Name[:]),
			Terminal: common.IntToString(u.Line[:]),
			Host:     common.IntToString(u.Host[:]),
			Started:  int(u.Time),
		}

		ret = append(ret, user)
	}

	return ret, nil
}

func KernelVersionWithContext(ctx context.Context) (string, error) {
	_, _, version, err := PlatformInformationWithContext(ctx)
	return version, err
}
