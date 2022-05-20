package blackbox_tests_test

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kumahq/kuma-net/iptables/builder"
	"github.com/kumahq/kuma-net/iptables/config"
	"github.com/kumahq/kuma-net/iptables/consts"
	"github.com/kumahq/kuma-net/test/blackbox_tests"
	"github.com/kumahq/kuma-net/test/framework/netns"
	"github.com/kumahq/kuma-net/test/framework/socket"
	"github.com/kumahq/kuma-net/test/framework/syscall"
	"github.com/kumahq/kuma-net/test/framework/sysctl"
	"github.com/kumahq/kuma-net/test/framework/tcp"
	"github.com/kumahq/kuma-net/test/framework/udp"
)

var _ = Describe("Outbound IPv4 DNS/UDP traffic to port 53", func() {
	var err error
	var ns *netns.NetNS

	BeforeEach(func() {
		ns, err = netns.NewNetNSBuilder().Build()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(ns.Cleanup()).To(Succeed())
	})

	DescribeTable("should be redirected to provided port",
		func(randomPort uint16) {
			// given
			address := udp.GenRandomAddressIPv4(consts.DNSPort)
			tproxyConfig := config.Config{
				Redirect: config.Redirect{
					DNS: config.DNS{
						Enabled: true,
						Port:    randomPort,
					},
				},
				RuntimeOutput: ioutil.Discard,
			}
			serverAddress := fmt.Sprintf("%s:%d", consts.LocalhostIPv4, randomPort)

			readyC, errC := udp.UnsafeStartUDPServer(ns, serverAddress, udp.ReplyWithReceivedMsg)
			Consistently(errC).ShouldNot(Receive())
			Eventually(readyC).Should(BeClosed())

			// when
			Eventually(ns.UnsafeExec(func() {
				Expect(builder.RestoreIPTables(tproxyConfig)).Error().To(Succeed())
			})).Should(BeClosed())

			// and
			Eventually(ns.UnsafeExec(func() {
				Expect(udp.DialUDPAddrWithHelloMsgAndGetReply(address, address)).
					To(Equal(address.String()))
			})).Should(BeClosed())

			// then
			Consistently(errC).ShouldNot(Receive())
		},
		func() []TableEntry {
			var entries []TableEntry
			lockedPorts := []uint16{consts.DNSPort}

			for i := 0; i < blackbox_tests.TestCasesAmount; i++ {
				randomPorts := socket.GenerateRandomPortsSlice(1, lockedPorts...)
				// This gives us more entropy as all generated ports will be
				// different from each other
				lockedPorts = append(lockedPorts, randomPorts...)
				desc := fmt.Sprintf("to port %%d, from port %d", consts.DNSPort)
				entry := Entry(EntryDescription(desc), randomPorts[0])
				entries = append(entries, entry)
			}

			return entries
		}(),
	)
})

var _ = Describe("Outbound IPv4 DNS/TCP traffic to port 53", func() {
	var err error
	var ns *netns.NetNS

	BeforeEach(func() {
		ns, err = netns.NewNetNSBuilder().Build()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(ns.Cleanup()).To(Succeed())
	})

	DescribeTable("should be redirected to provided port",
		func(dnsPort, outboundPort uint16) {
			// given
			address := tcp.GenRandomAddressIPv4(consts.DNSPort)
			tproxyConfig := config.Config{
				Redirect: config.Redirect{
					DNS: config.DNS{
						Enabled: true,
						Port:    dnsPort,
					},
					Outbound: config.TrafficFlow{
						Port: outboundPort,
					},
				},
				RuntimeOutput: ioutil.Discard,
			}
			serverAddress := fmt.Sprintf("%s:%d", consts.LocalhostIPv4, dnsPort)

			readyC, errC := tcp.UnsafeStartTCPServer(
				ns,
				serverAddress,
				tcp.ReplyWithOriginalDstIPv4,
				tcp.CloseConn,
			)
			Consistently(errC).ShouldNot(Receive())
			Eventually(readyC).Should(BeClosed())

			// when
			Eventually(ns.UnsafeExec(func() {
				Expect(builder.RestoreIPTables(tproxyConfig)).Error().To(Succeed())
			})).Should(BeClosed())

			// and
			Eventually(ns.UnsafeExec(func() {
				Expect(tcp.DialTCPAddrAndGetReply(address)).To(Equal(address.String()))
			})).Should(BeClosed())

			// then
			Eventually(errC).Should(BeClosed())
		},
		func() []TableEntry {
			var entries []TableEntry
			lockedPorts := []uint16{consts.DNSPort}

			for i := 0; i < blackbox_tests.TestCasesAmount; i++ {
				// We are drawing two ports instead of one as the first one will be used
				// to expose TCP server inside the namespace, which will be pretending
				// a DNS server which should intercept all DNS traffic on port TCP#53,
				// and the second one will be set as an outbound redirection port,
				// which wound intercept the packet if no DNS redirection would be set,
				// and we don't want them to be the same
				randomPorts := socket.GenerateRandomPortsSlice(2, lockedPorts...)
				// This gives us more entropy as all generated ports will be
				// different from each other
				lockedPorts = append(lockedPorts, randomPorts...)
				desc := fmt.Sprintf(
					"to port %d, from port %d",
					randomPorts[0],
					consts.DNSPort,
				)
				entry := Entry(EntryDescription(desc), randomPorts[0], randomPorts[1])
				entries = append(entries, entry)
			}

			return entries
		}(),
	)
})

var _ = Describe("Outbound IPv6 DNS/UDP traffic to port 53", func() {
	var err error
	var ns *netns.NetNS

	BeforeEach(func() {
		ns, err = netns.NewNetNSBuilder().WithIPv6(true).Build()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(ns.Cleanup()).To(Succeed())
	})

	DescribeTable("should be redirected to provided port",
		func(randomPort uint16) {
			// given
			address := udp.GenRandomAddressIPv6(consts.DNSPort)
			tproxyConfig := config.Config{
				Redirect: config.Redirect{
					DNS: config.DNS{
						Enabled: true,
						Port:    randomPort,
					},
				},
				IPv6:          true,
				RuntimeOutput: ioutil.Discard,
			}
			serverAddress := fmt.Sprintf("%s:%d", consts.LocalhostIPv6, randomPort)

			readyC, errC := udp.UnsafeStartUDPServer(ns, serverAddress, udp.ReplyWithReceivedMsg)
			Consistently(errC).ShouldNot(Receive())
			Eventually(readyC).Should(BeClosed())

			// when
			Eventually(ns.UnsafeExec(func() {
				Expect(builder.RestoreIPTables(tproxyConfig)).Error().To(Succeed())
			})).Should(BeClosed())

			// and
			Eventually(ns.UnsafeExec(func() {
				Expect(udp.DialUDPAddrWithHelloMsgAndGetReply(address, address)).
					To(Equal(address.String()))
			})).Should(BeClosed())

			// then
			Consistently(errC).ShouldNot(Receive())
		},
		func() []TableEntry {
			var entries []TableEntry
			lockedPorts := []uint16{consts.DNSPort}

			for i := 0; i < blackbox_tests.TestCasesAmount; i++ {
				randomPorts := socket.GenerateRandomPortsSlice(1, lockedPorts...)
				// This gives us more entropy as all generated ports will be
				// different from each other
				lockedPorts = append(lockedPorts, randomPorts...)
				desc := fmt.Sprintf("to port %%d, from port %d", consts.DNSPort)
				entry := Entry(EntryDescription(desc), randomPorts[0])
				entries = append(entries, entry)
			}

			return entries
		}(),
	)
})

var _ = Describe("Outbound IPv6 DNS/TCP traffic to port 53", func() {
	var err error
	var ns *netns.NetNS

	BeforeEach(func() {
		ns, err = netns.NewNetNSBuilder().WithIPv6(true).Build()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(ns.Cleanup()).To(Succeed())
	})

	DescribeTable("should be redirected to provided port",
		func(dnsPort, outboundPort uint16) {
			// given
			address := tcp.GenRandomAddressIPv6(consts.DNSPort)
			tproxyConfig := config.Config{
				Redirect: config.Redirect{
					DNS: config.DNS{
						Enabled: true,
						Port:    dnsPort,
					},
					Outbound: config.TrafficFlow{
						Port: outboundPort,
					},
				},
				IPv6:          true,
				RuntimeOutput: ioutil.Discard,
			}
			serverAddress := fmt.Sprintf("%s:%d", consts.LocalhostIPv6, dnsPort)

			readyC, errC := tcp.UnsafeStartTCPServer(
				ns,
				serverAddress,
				tcp.ReplyWithOriginalDstIPv6,
				tcp.CloseConn,
			)
			Consistently(errC).ShouldNot(Receive())
			Eventually(readyC).Should(BeClosed())

			// when
			Eventually(ns.UnsafeExec(func() {
				Expect(builder.RestoreIPTables(tproxyConfig)).Error().To(Succeed())
			})).Should(BeClosed())

			// and
			Eventually(ns.UnsafeExec(func() {
				Expect(tcp.DialTCPAddrAndGetReply(address)).To(Equal(address.String()))
			})).Should(BeClosed())

			// then
			Eventually(errC).Should(BeClosed())
		},
		func() []TableEntry {
			var entries []TableEntry
			lockedPorts := []uint16{consts.DNSPort}

			for i := 0; i < blackbox_tests.TestCasesAmount; i++ {
				// We are drawing two ports instead of one as the first one will be used
				// to expose TCP server inside the namespace, which will be pretending
				// a DNS server which should intercept all DNS traffic on port TCP#53,
				// and the second one will be set as an outbound redirection port,
				// which wound intercept the packet if no DNS redirection would be set,
				// and we don't want them to be the same
				randomPorts := socket.GenerateRandomPortsSlice(2, lockedPorts...)
				// This gives us more entropy as all generated ports will be
				// different from each other
				lockedPorts = append(lockedPorts, randomPorts...)
				desc := fmt.Sprintf(
					"to port %d, from port %d",
					randomPorts[0],
					consts.DNSPort,
				)
				entry := Entry(EntryDescription(desc), randomPorts[0], randomPorts[1])
				entries = append(entries, entry)
			}

			return entries
		}(),
	)
})

var _ = Describe("Outbound IPv4 DNS/UDP conntrack zone splitting", func() {
	var err error
	var ns *netns.NetNS

	BeforeEach(func() {
		ns, err = netns.NewNetNSBuilder().
			WithBeforeExecFuncs(sysctl.SetLocalPortRange(32768, 32770)).
			Build()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(ns.Cleanup()).To(Succeed())
	})

	DescribeTable("should be redirected to provided port",
		func(port uint16) {
			// given
			uid := uintptr(5678)
			s1Address := fmt.Sprintf("%s:%d", ns.Veth().PeerAddress(), consts.DNSPort)
			s2Address := fmt.Sprintf("%s:%d", consts.LocalhostIPv4, port)
			tproxyConfig := config.Config{
				Redirect: config.Redirect{
					DNS: config.DNS{
						Enabled:            true,
						Port:               port,
						ConntrackZoneSplit: true,
					},
				},
				Owner:         config.Owner{UID: strconv.Itoa(int(uid))},
				RuntimeOutput: ioutil.Discard,
			}
			want := map[string]uint{
				s1Address: blackbox_tests.DNSConntrackZoneSplittingStressCallsAmount,
				s2Address: blackbox_tests.DNSConntrackZoneSplittingStressCallsAmount,
			}

			s1ReadyC, s1ErrC := udp.UnsafeStartUDPServer(
				ns,
				s1Address,
				udp.ReplyWithLocalAddr,
			)
			Consistently(s1ErrC).ShouldNot(Receive())
			Eventually(s1ReadyC).Should(BeClosed())

			s2ReadyC, s2ErrC := udp.UnsafeStartUDPServer(
				ns,
				s2Address,
				udp.ReplyWithLocalAddr,
				sysctl.SetUnprivilegedPortStart(0),
				syscall.SetUID(uid),
			)
			Consistently(s2ErrC).ShouldNot(Receive())
			Eventually(s2ReadyC).Should(BeClosed())

			// when
			Eventually(ns.UnsafeExec(func() {
				Expect(builder.RestoreIPTables(tproxyConfig)).Error().To(Succeed())
			})).Should(BeClosed())

			results := udp.NewResultMap()

			exec1ErrC := ns.UnsafeExecInLoop(
				blackbox_tests.DNSConntrackZoneSplittingStressCallsAmount,
				time.Millisecond,
				func() {
					Expect(udp.DialAddrAndIncreaseResultMap(s1Address, results)).To(Succeed())
				},
				syscall.SetUID(uid),
			)

			exec2ErrC := ns.UnsafeExecInLoop(
				blackbox_tests.DNSConntrackZoneSplittingStressCallsAmount,
				time.Millisecond,
				func() {
					Expect(udp.DialAddrAndIncreaseResultMap(s1Address, results)).To(Succeed())
				},
			)

			Consistently(exec1ErrC).ShouldNot(Receive())
			Consistently(exec2ErrC).ShouldNot(Receive())
			Eventually(exec1ErrC, blackbox_tests.DNSConntrackZoneSplittingTestTimeout).
				Should(BeClosed())
			Eventually(exec2ErrC, blackbox_tests.DNSConntrackZoneSplittingTestTimeout).
				Should(BeClosed())

			Expect(results.GetFinalResults()).To(BeEquivalentTo(want))
		},
		func() []TableEntry {
			var entries []TableEntry
			lockedPorts := []uint16{consts.DNSPort}

			for i := 0; i < blackbox_tests.TestCasesAmount; i++ {
				ports := socket.GenerateRandomPortsSlice(1, lockedPorts...)
				// This gives us more entropy as all generated ports will be
				// different from each other
				lockedPorts = append(lockedPorts, ports...)
				desc := fmt.Sprintf("to port %%d, from port %d", consts.DNSPort)
				entry := Entry(EntryDescription(desc), ports[0])
				entries = append(entries, entry)
			}

			return entries
		}(),
	)
})

var _ = Describe("Outbound IPv6 DNS/UDP conntrack zone splitting", func() {
	var err error
	var ns *netns.NetNS

	BeforeEach(func() {
		ns, err = netns.NewNetNSBuilder().
			WithIPv6(true).
			WithBeforeExecFuncs(sysctl.SetLocalPortRange(32768, 32770)).
			Build()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(ns.Cleanup()).To(Succeed())
	})

	DescribeTable("should be redirected to provided port",
		func(port uint16) {
			// given
			uid := uintptr(5678)
			s1Address := fmt.Sprintf("%s:%d", consts.LocalhostIPv6, consts.DNSPort)
			s2Address := fmt.Sprintf("%s:%d", consts.LocalhostIPv6, port)
			tproxyConfig := config.Config{
				Redirect: config.Redirect{
					DNS: config.DNS{
						Enabled:            true,
						Port:               port,
						ConntrackZoneSplit: true,
					},
				},
				IPv6:          true,
				Owner:         config.Owner{UID: strconv.Itoa(int(uid))},
				RuntimeOutput: ioutil.Discard,
			}
			want := map[string]uint{
				s1Address: blackbox_tests.DNSConntrackZoneSplittingStressCallsAmount,
				s2Address: blackbox_tests.DNSConntrackZoneSplittingStressCallsAmount,
			}

			s1ReadyC, s1ErrC := udp.UnsafeStartUDPServer(
				ns,
				s1Address,
				udp.ReplyWithLocalAddr,
			)
			Consistently(s1ErrC).ShouldNot(Receive())
			Eventually(s1ReadyC).Should(BeClosed())

			s2ReadyC, s2ErrC := udp.UnsafeStartUDPServer(
				ns,
				s2Address,
				udp.ReplyWithLocalAddr,
				sysctl.SetUnprivilegedPortStart(0),
				syscall.SetUID(uid),
			)
			Consistently(s2ErrC).ShouldNot(Receive())
			Eventually(s2ReadyC).Should(BeClosed())

			// when
			Eventually(ns.UnsafeExec(func() {
				Expect(builder.RestoreIPTables(tproxyConfig)).Error().To(Succeed())
			})).Should(BeClosed())

			results := udp.NewResultMap()

			exec1ErrC := ns.UnsafeExecInLoop(
				blackbox_tests.DNSConntrackZoneSplittingStressCallsAmount,
				time.Millisecond,
				func() {
					Expect(udp.DialAddrAndIncreaseResultMap(s1Address, results)).To(Succeed())
				},
				syscall.SetUID(uid),
			)

			exec2ErrC := ns.UnsafeExecInLoop(
				blackbox_tests.DNSConntrackZoneSplittingStressCallsAmount,
				time.Millisecond,
				func() {
					Expect(udp.DialAddrAndIncreaseResultMap(s1Address, results)).To(Succeed())
				},
			)

			Consistently(exec1ErrC).ShouldNot(Receive())
			Consistently(exec2ErrC).ShouldNot(Receive())
			Eventually(exec1ErrC, blackbox_tests.DNSConntrackZoneSplittingTestTimeout).
				Should(BeClosed())
			Eventually(exec2ErrC, blackbox_tests.DNSConntrackZoneSplittingTestTimeout).
				Should(BeClosed())

			Expect(results.GetFinalResults()).To(BeEquivalentTo(want))
		},
		func() []TableEntry {
			var entries []TableEntry
			lockedPorts := []uint16{consts.DNSPort}

			for i := 0; i < blackbox_tests.TestCasesAmount; i++ {
				ports := socket.GenerateRandomPortsSlice(1, lockedPorts...)
				// This gives us more entropy as all generated ports will be
				// different from each other
				lockedPorts = append(lockedPorts, ports...)
				desc := fmt.Sprintf("to port %%d, from port %d", consts.DNSPort)
				entry := Entry(EntryDescription(desc), ports[0])
				entries = append(entries, entry)
			}

			return entries
		}(),
	)
})
