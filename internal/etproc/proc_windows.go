package etproc

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// sysProcAttr starts the child without a console window.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

var (
	jobOnce sync.Once
	job     windows.Handle
	jobErr  error
)

// killOnCloseJob is a job object that kills every process in it when its
// last handle closes, i.e. when this process exits for whatever reason. It
// is the Windows counterpart of Linux's parent-death signal.
func killOnCloseJob() (windows.Handle, error) {
	jobOnce.Do(func() {
		job, jobErr = windows.CreateJobObject(nil, nil)
		if jobErr != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		_, jobErr = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	})
	return job, jobErr
}

// afterStart puts the child (and anything it spawns) into the job.
func afterStart(cmd *exec.Cmd) error {
	j, err := killOnCloseJob()
	if err != nil {
		return fmt.Errorf("create job object: %w", err)
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fmt.Errorf("open child process: %w", err)
	}
	defer windows.CloseHandle(h)
	if err := windows.AssignProcessToJobObject(j, h); err != nil {
		return fmt.Errorf("assign child to job object: %w", err)
	}
	return nil
}

// Windows has no SIGTERM for a console-less child: terminate kills it.
// easytier-core removes its Wintun adapter when its handles close.
func terminate(pid int) error { return kill(pid) }

func kill(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
