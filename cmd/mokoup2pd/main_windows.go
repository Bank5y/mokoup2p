package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/Bank5y/mokoup2p/client/daemon"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

const (
	serviceName = "mokoup2p"
	usage       = `用法：
  mokoup2pd install [-user 用户名]  注册并启动 Windows 服务（需要管理员权限）；
                                   -user 是可以使用客户端的用户，默认为当前用户
  mokoup2pd uninstall              停止并删除 Windows 服务
  mokoup2pd run                    在控制台中运行（调试用）
`
)

func main() {
	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("detect service: %v", err)
	}
	if isService {
		runService()
		return
	}
	cmd := "run"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "install":
		fs := flag.NewFlagSet("install", flag.ExitOnError)
		name := fs.String("user", "", "user allowed to use the client (default: current user)")
		_ = fs.Parse(os.Args[2:])
		err = install(*name)
	case "uninstall":
		err = uninstall()
	case "run":
		err = runConsole()
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

// serviceStateDir keeps the service's state machine-wide.
func serviceStateDir() string {
	if dir := os.Getenv("MOKOUP2P_STATE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(os.Getenv("ProgramData"), "mokoup2p")
}

// runConsole runs the daemon for the current user, in the foreground.
func runConsole() error {
	stateDir := os.Getenv("MOKOUP2P_STATE_DIR")
	if stateDir == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		stateDir = filepath.Join(cfgDir, "mokoup2p")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return run(ctx, stateDir, func() (net.Listener, string, error) {
		pipe := ipc.DefaultSocketPath()
		l, err := daemon.Listen(pipe)
		return l, pipe, err
	})
}

// runService runs under the service control manager. The service was
// registered with `-ipc-user <SID>`: the only user its pipe admits.
func runService() {
	fs := flag.NewFlagSet("service", flag.ContinueOnError)
	sid := fs.String("ipc-user", "", "SID of the user allowed on the IPC pipe")
	_ = fs.Parse(os.Args[1:])
	stateDir := serviceStateDir()
	if err := os.MkdirAll(stateDir, 0o700); err == nil {
		if f, err := os.OpenFile(filepath.Join(stateDir, "mokoup2pd.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			log.SetOutput(f)
		}
	}
	if err := svc.Run(serviceName, &service{stateDir: stateDir, sid: *sid}); err != nil {
		log.Printf("service: %v", err)
	}
}

type service struct {
	stateDir string
	sid      string
}

func (s *service) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	if s.sid == "" {
		log.Print("service: started without -ipc-user; reinstall with `mokoup2pd install`")
		return true, 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, s.stateDir, func() (net.Listener, string, error) {
			l, err := daemon.ListenPipe(ipc.DefaultPipeName, s.sid)
			return l, ipc.DefaultPipeName, err
		})
	}()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				status <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// Stopping leaves the room and stops the engine (Daemon.Close).
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		case err := <-done:
			if err != nil {
				log.Print(err)
				return true, 1
			}
			return false, 0
		}
	}
}

// install registers the service for the given user (default: whoever runs
// the installer) and starts it.
func install(username string) error {
	sid, err := userSID(username)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to the service manager (run as administrator): %w", err)
	}
	defer m.Disconnect()
	if s, err := m.OpenService(serviceName); err == nil {
		s.Close()
		return errors.New("service already installed; run `mokoup2pd uninstall` first")
	}
	s, err := m.CreateService(serviceName, exe, mgr.Config{
		DisplayName: "mokoup2p",
		Description: "mokoup2p 虚拟局域网后台服务",
		StartType:   mgr.StartAutomatic,
	}, "-ipc-user", sid)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()
	_ = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
	}, 24*60*60)
	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	fmt.Printf("已注册并启动服务 %s（允许用户 %s）\n", serviceName, sid)
	return nil
}

func userSID(username string) (string, error) {
	if username != "" {
		return daemon.LookupUserSID(username)
	}
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Uid, nil
}

// uninstall stops (leaving any room) and deletes the service.
func uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to the service manager (run as administrator): %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service not installed: %w", err)
	}
	defer s.Close()
	if st, err := s.Control(svc.Stop); err == nil {
		for deadline := time.Now().Add(20 * time.Second); st.State != svc.Stopped && time.Now().Before(deadline); {
			time.Sleep(300 * time.Millisecond)
			if st, err = s.Query(); err != nil {
				break
			}
		}
	}
	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	fmt.Printf("已删除服务 %s\n", serviceName)
	return nil
}
