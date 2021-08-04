package main

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	socks5 "golang.org/x/net/proxy"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"sync"
	"syscall"
	"time"
)

var getProxyUrl = ""

func init() {
	go func() {
		log.Fatal(http.ListenAndServe(":9876", nil))
	}()
}

var ipMap = sync.Map{}

func main() {
	//go httpListen()
	go checkProxy()
	if l, err := net.Listen("tcp", ":8082"); err == nil {
		log.Infof("begin")
		for {
			if conn, err := l.Accept(); err == nil {
				go handle(conn)
			} else {
				log.Errorf("AcceptFail;err:%s", err.Error())
			}
		}
	} else {
		log.Panicf("listenPanic;err:%s", err.Error())
	}
}

var counter int64 = 0

func handle(conn net.Conn) {
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Errorf("connCloseFail;err:%s", err.Error())
		}
	}()

	// getOriginalDst
	remote, err := getOriginalDst(conn)
	if err != nil {
		log.Errorf("getOriginalDstFail;err:%s", err.Error())
		return
	}
	src := conn.RemoteAddr().String()
	var proxy = ProxyIp{}
	var dialer socks5.Dialer

	var c net.Conn
	for i := 1; i <= 30; i++ {
		// build a dialer via proxyIp
		proxy = getProxy("", "")
		dialer, err = socks5.SOCKS5("tcp", proxy.ProxyIp, &socks5.Auth{User: proxy.Auth, Password: proxy.Password}, &net.Dialer{Timeout: 3 * time.Second})
		if err != nil {
			log.Errorf("buildDialerFail;err:%s", err.Error())
			return
		}

		// dial remote address
		c, err = dialer.Dial("tcp", remote)
		if err != nil {
			if i == 30 {
				log.Errorf("dialRemoteFail;retryTimes:%d;err:%s", i, err.Error())
				// dial remote max retry
				return
			}
			continue
		}
		break
	}

	// log.Infof("begin;localIp:%s;handle;proxy:%s;remote:%s", src, proxy.ProxyIp, remote)
	ExitChan := make(chan bool, 1)
	go func() {
		_, _ = io.Copy(c, conn)
		ExitChan <- true
	}()
	go func() {
		_, _ = io.Copy(conn, c)
		ExitChan <- true
	}()
	<-ExitChan
	err = c.Close()
	if err != nil {
		log.Errorf("cCloseFail;localIp:%s;err:%s", src, err.Error())
		return
	}
	// log.Infof("end;localIp:%s;handle;proxy:%s;remote:%s", src, proxy.ProxyIp, remote)
	return
}

func getOriginalDst(client net.Conn) (string, error) {
	clientTcp, ok := client.(*net.TCPConn)
	if !ok {
		return "", errors.New("assertNetTcpConnFail")
	}
	clientFile, err := clientTcp.File()
	if err != nil {
		return "", err
	}

	defer clientFile.Close()
	fd := clientFile.Fd()

	addr, err := syscall.GetsockoptIPv6Mreq(int(fd), syscall.IPPROTO_IP, 80)
	if err != nil {
		return "", err
	}

	remote := fmt.Sprintf("%d.%d.%d.%d:%d",
		addr.Multiaddr[4],
		addr.Multiaddr[5],
		addr.Multiaddr[6],
		addr.Multiaddr[7],
		uint16(addr.Multiaddr[2])<<8+uint16(addr.Multiaddr[3]))
	return remote, nil
}

type ProxyIp struct {
	Ip           string
	Port         int64
	Auth         string
	Password     string
	ProxyIp      string
	EndTimestamp int64
}

type GetProxyResponseBody struct {
	Code int `json:"code"`
	Data struct {
		ProxyIP      string `json:"proxyIp"`
		ProxyPort    int64  `json:"proxyPort"`
		ProxyAddr    string `json:"proxyAddr"`
		EndTimestamp int64  `json:"endTimestamp"`
		OriginIP     string `json:"originIp"`
		ProxyID      string `json:"proxyId"`
		ProxySecret  string `json:"proxySecret"`
	} `json:"data"`
	Msg string `json:"msg"`
}

var PhoneIpMapProxyIp = map[string]ProxyIp{}
var PhoneIpMapProxyIpLock = &sync.Mutex{}

type Ac struct {
	ProxyIP      string `json:"proxyIp"`
	ProxyPort    int64  `json:"proxyPort"`
	ProxyAddr    string `json:"proxyAddr"`
	EndTimestamp int64  `json:"endTimestamp"`
	OriginIP     string `json:"originIp"`
	ProxyID      string `json:"proxyId"`
	ProxySecret  string `json:"proxySecret"`
	Aaaaa        string `json:"aaaaa"`
}

var proxyMap = map[string]Ac{}

func getProxy(src string, forceGet string) (ret ProxyIp) {
	return ProxyIp{
		Ip:           "tunnel.qg.net",
		Port:         22749,
		Auth:         "C9B05C89",
		Password:     "26BA01B5CB28",
		ProxyIp:      "tunnel.qg.net",
		EndTimestamp: 99999999,
	}
	/*
		PhoneIpMapProxyIpLock.Lock()
		defer PhoneIpMapProxyIpLock.Unlock()
		for _, v := range proxyMap {
			ret.Ip = v.ProxyIP
			ret.Port = v.ProxyPort
			ret.ProxyIp = v.ProxyAddr
			ret.Auth = v.ProxyID
			ret.Password = v.ProxySecret
			ret.EndTimestamp = 99999999
		}
		return ret

	*/
}

func index(w http.ResponseWriter, r *http.Request) {
	raddr := strings.Split(r.RemoteAddr, ":")
	ipMap.Delete(raddr[0])
	w.Write([]byte("ok"))

}

func httpListen() {
	http.HandleFunc("/", index)
	http.HandleFunc("/getLocalIp", func(w http.ResponseWriter, r *http.Request) {
		if r.RemoteAddr != "" {
			ipList := strings.Split(r.RemoteAddr, ":")
			if len(ipList) < 2 {
				log.Printf("remoteAddrLenFail;remoteAddr:%s", r.RemoteAddr)
				return
			}
			w.Write([]byte(ipList[0]))
			return
		} else {
			log.Printf("getLocalIpFail;")
		}

		return
	})
	http.HandleFunc("/counter", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("counter:%d", counter)))
		return
	})
	if err := http.ListenAndServe(":8888", nil); err != nil {
		log.Fatal(err)
	}

}

func testProxy(p ProxyIp) bool {
	// create a socks5 dialer
	dialer, err := socks5.SOCKS5("tcp", p.ProxyIp, &socks5.Auth{User: p.Auth, Password: p.Password}, socks5.Direct)
	if err != nil {
		log.Errorf("ProxyConnectFail;p:%+v;err:%s", p, err.Error())
		return false
	}
	// setup a http client
	httpTransport := &http.Transport{}
	httpClient := &http.Client{Transport: httpTransport}
	// set our socks5 as the dialer
	httpTransport.Dial = dialer.Dial
	if resp, err := httpClient.Get("https://ecs.aliyuncs.com"); err != nil {
		log.Errorf("ProxyGetFail;p:%+v;err:%s", p, err.Error())
		return false
	} else {
		_ = resp.Body.Close()
		return true
	}
}

func checkProxy() {
	for {
		updateProxy()
		time.Sleep(3 * time.Second)
	}
}

func updateProxy() {
	kk, err := http.Get(getProxyUrl)
	if err != nil {
		log.Printf("e1:", err.Error())
		return
	}
	bB, err := ioutil.ReadAll(kk.Body)
	defer kk.Body.Close()
	if err != nil {
		log.Printf("e2:", err.Error())
		return
	}
	PhoneIpMapProxyIpLock.Lock()
	defer PhoneIpMapProxyIpLock.Unlock()
	proxyMap = nil
	err = json.Unmarshal(bB, &proxyMap)
	if err != nil {
		log.Printf("e3:", err.Error())
		return
	}
	log.Printf("updateProxySuc")
}
