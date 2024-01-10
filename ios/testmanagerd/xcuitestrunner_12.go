package testmanagerd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/nskeyedarchiver"
	log "github.com/sirupsen/logrus"
)

func RunXUITestWithBundleIdsXcode12Ctx(ctx context.Context, bundleID string, testRunnerBundleID string, xctestConfigFileName string,
	device ios.DeviceEntry, args []string, env []string, testListener *TestListener,
) error {
	conn, err := dtx.NewUsbmuxdConnection(device, testmanagerdiOS14)
	if err != nil {
		return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot create a usbmuxd connection to testmanagerd: %w", err)
	}

	testSessionId, xctestConfigPath, testConfig, testInfo, err := setupXcuiTest(device, bundleID, testRunnerBundleID, xctestConfigFileName)
	if err != nil {
		return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot setup test config: %w", err)
	}
	defer conn.Close()
	ideDaemonProxy := newDtxProxyWithConfig(conn, testConfig, testListener)

	conn2, err := dtx.NewUsbmuxdConnection(device, testmanagerdiOS14)
	if err != nil {
		return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot create a usbmuxd connection to testmanagerd: %w", err)
	}
	defer conn2.Close()
	log.Debug("connections ready")
	ideDaemonProxy2 := newDtxProxyWithConfig(conn2, testConfig, testListener)
	ideDaemonProxy2.ideInterface.testConfig = testConfig
	caps, err := ideDaemonProxy.daemonConnection.initiateControlSessionWithCapabilities(nskeyedarchiver.XCTCapabilities{})
	if err != nil {
		return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot initiate a control session with capabilities: %w", err)
	}
	log.Debug(caps)
	localCaps := nskeyedarchiver.XCTCapabilities{CapabilitiesDictionary: map[string]interface{}{
		"XCTIssue capability":     uint64(1),
		"skipped test capability": uint64(1),
		"test timeout capability": uint64(1),
	}}

	caps2, err := ideDaemonProxy2.daemonConnection.initiateSessionWithIdentifierAndCaps(testSessionId, localCaps)
	if err != nil {
		return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot initiate a session with identifier and capabilities: %w", err)
	}
	log.Debug(caps2)
	pControl, err := instruments.NewProcessControl(device)
	if err != nil {
		return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot connect to process control: %w", err)
	}
	defer pControl.Close()

	pid, err := startTestRunner12(pControl, xctestConfigPath, testRunnerBundleID, testSessionId.String(), testInfo.testrunnerAppPath+"/PlugIns/"+xctestConfigFileName, args, env)
	if err != nil {
		return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot start test runner: %w", err)
	}
	log.Debugf("Runner started with pid:%d, waiting for testBundleReady", pid)

	proxyDispatcher := ProxyDispatcher{id: "emty"}
	ideInterfaceChannel := ideDaemonProxy2.dtxConnection.ForChannelRequest(proxyDispatcher)

	time.Sleep(time.Second)

	success, _ := ideDaemonProxy.daemonConnection.authorizeTestSessionWithProcessID(pid)
	log.Debugf("authorizing test session for pid %d successful %t", pid, success)
	err = ideDaemonProxy2.daemonConnection.startExecutingTestPlanWithProtocolVersion(ideInterfaceChannel, 36)
	if err != nil {
		log.Error(err)
		return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot authorize test session: %w", err)
	}

	if ctx != nil {
		select {
		case <-ctx.Done():
			log.Infof("Killing test runner with pid %d ...", pid)
			err = pControl.KillProcess(pid)
			if err != nil {
				return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot kill test runner: %w", err)
			}
			log.Info("Test runner killed with success")
		}
		return nil
	}

	<-proxyDispatcher.closeChannel
	log.Debugf("Killing UITest with pid %d ...", pid)
	err = pControl.KillProcess(pid)
	if err != nil {
		return fmt.Errorf("RunXUITestWithBundleIdsXcode12Ctx: cannot kill test runner: %w", err)
	}
	log.Debugf("Test runner killed with success")
	var signal interface{}
	proxyDispatcher.closedChannel <- signal
	return nil
}

func startTestRunner12(pControl *instruments.ProcessControl, xctestConfigPath string, bundleID string,
	sessionIdentifier string, testBundlePath string, wdaargs []string, wdaenv []string,
) (uint64, error) {
	args := []interface{}{
		"-NSTreatUnknownArgumentsAsOpen", "NO", "-ApplePersistenceIgnoreState", "YES",
	}
	for _, arg := range wdaargs {
		args = append(args, arg)
	}
	env := map[string]interface{}{
		"CA_ASSERT_MAIN_THREAD_TRANSACTIONS": "0",
		"CA_DEBUG_TRANSACTIONS":              "0",
		"DYLD_INSERT_LIBRARIES":              "/Developer/usr/lib/libMainThreadChecker.dylib",

		"MTC_CRASH_ON_REPORT":             "1",
		"NSUnbufferedIO":                  "YES",
		"OS_ACTIVITY_DT_MODE":             "YES",
		"SQLITE_ENABLE_THREAD_ASSERTIONS": "1",
		"XCTestBundlePath":                testBundlePath,
		"XCTestConfigurationFilePath":     xctestConfigPath,
		"XCTestSessionIdentifier":         sessionIdentifier,
	}

	for _, entrystring := range wdaenv {
		entry := strings.Split(entrystring, "=")
		key := entry[0]
		value := entry[1]
		env[key] = value
		log.Debugf("adding extra env %s=%s", key, value)
	}

	opts := map[string]interface{}{
		"StartSuspendedKey": uint64(0),
		"ActivateSuspended": uint64(1),
	}

	return pControl.StartProcess(bundleID, env, args, opts)
}
