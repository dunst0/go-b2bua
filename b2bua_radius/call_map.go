//
// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2014 Sippy Software, Inc. All rights reserved.
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification,
// are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
// list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation and/or
// other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
// ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
package main

import (
    "fmt"
    "os"
    "os/exec"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "sippy/headers"
    "sippy/types"
)

type callMap struct {
    global_config   *myConfigParser
    ccmap           map[int64]*callController
    gc_timeout      time.Duration
    debug_mode      bool
    safe_restart    bool
    sip_tm          sippy_types.SipTransactionManager
    proxy           sippy_types.StatefulProxy
    cc_id           int64
    cc_id_lock      sync.Mutex
}

/*
class CallMap(object):
    ccmap = nil
    el = nil
    global_config = nil
    //rc1 = nil
    //rc2 = nil
*/

func NewCallMap(global_config *myConfigParser) *callMap {
    self := &callMap{
        global_config   : global_config,
        ccmap           : make(map[int64]*callController),
        gc_timeout      : time.Minute,
        debug_mode      : false,
        safe_restart    : false,
    }
    go func() {
        sighup_ch := make(chan os.Signal, 1)
        signal.Notify(sighup_ch, syscall.SIGHUP)
        sigusr2_ch := make(chan os.Signal, 1)
        signal.Notify(sigusr2_ch, syscall.SIGUSR2)
        sigprof_ch := make(chan os.Signal, 1)
        signal.Notify(sigprof_ch, syscall.SIGPROF)
        for {
            select {
            case <-sighup_ch:
                self.discAll(syscall.SIGHUP)
            case <-sigusr2_ch:
                self.toggleDebug()
            case <-sigprof_ch:
                self.safeRestart()
            }
        }
    }()
    go func() {
        for {
            time.Sleep(self.gc_timeout)
            self.GClector()
        }
    }()
    return self
}

func (self *callMap) OnNewDialog(req sippy_types.SipRequest, sip_t sippy_types.ServerTransaction) {
    to_tag := req.GetTo().GetTag()
    //except Exception as exception:
        //println(datetime.now(), "can\"t parse SIP request: %s:\n" % str(exception))
        //println( "-" * 70)
        //print_exc(file = sys.stdout)
        //println( "-" * 70)
        //println(req)
        //println("-" * 70)
        //sys.stdout.flush()
        //return (nil, nil, nil)
    if to_tag != "" {
        // Request within dialog, but no such dialog
        return req.GenResponse(481, "Call Leg/Transaction Does Not Exist", nil, nil)
    }
    if req.GetMethod() == "INVITE" {
        // New dialog
        var via *sippy_header.SipVia
        vias := req.GetVias()
        if len(vias) > 1 {
            via = vias[1]
        } else {
            via = vias[0]
        }
        remote_ip := via.GetTAddr(self.global_config).Host
        source := req.GetSource()

        // First check if request comes from IP that
        // we want to accept our traffic from
        if ! self.global_config.checkIP(source.Host.String())  {
            resp := req.GenResponse(403, "Forbidden", nil, nil)
            return resp, nil, nil
        }
/*
        var challenge *sippy_header.SipWWWAuthenticate
        if self.global_config.auth_enable {
            // Prepare challenge if no authorization header is present.
            // Depending on configuration, we might try remote ip auth
            // first and then challenge it or challenge immediately.
            if self.global_config["digest_auth"] && req.countHFs("authorization") == 0 {
                challenge = NewSipWWWAuthenticate()
                challenge.getBody().realm = req.getRURI().host
            }
            // Send challenge immediately if digest is the
            // only method of authenticating
            if challenge != nil && self.global_config.getdefault("digest_auth_only", false) {
                resp = req.GenResponse(401, "Unauthorized")
                resp.appendHeader(challenge)
                return resp, nil, nil
            }
        }
*/
        pass_headers := []sippy_header.SipHeader{}
        for _, header := range self.global_config.pass_headers {
            hfs := req.GetHFs(header)
            pass_headers = append(pass_headers, hfs...)
        }
        self.cc_id_lock.Lock()
        id := self.cc_id
        self.cc_id++
        self.cc_id_lock.Unlock()
        cc := NewCallController(id, remote_ip, source, self.global_config, pass_headers)
        //cc.challenge = challenge
        rval = cc.uaA.recvRequest(req, sip_t)
        self.ccmap.append(cc)
        return rval
    }
    if self.proxy != nil && (req.GetMethod() == "REGISTER" || req.GetMethod() == "SUBSCRIBE") {
        return self.proxy.recvRequest(req)
    }
    if (req.GetMethod() == "NOTIFY" || req.GetMethod() == "PING") {
        // Whynot?
        return req.GenResponse(200, "OK", nil, nil)
    }
    return req.GenResponse(501, "Not Implemented", nil, nil)
}

func (self *callMap) discAll(signum syscall.Signal) {
    if signum > 0 {
        println(fmt.Sprintf("Signal %d received, disconnecting all calls", signum))
    }
    for _, cc := range self.ccmap {
        cc.disconnect()
    }
}

func (self *callMap) toggleDebug() {
    if self.debug_mode {
        println("Signal received, toggling extra debug output off")
    } else {
        println("Signal received, toggling extra debug output on")
    }
    self.debug_mode = ! self.debug_mode
}

func (self *callMap) safeRestart() {
    println("Signal received, scheduling safe restart")
    self.safe_restart = true
}

func (self *callMap) GClector() {
    fmt.Printf("GC is invoked, %d calls in map\n", len(self.ccmap))
    if self.debug_mode {
        //println(self.global_config["_sip_tm"].tclient, self.global_config["_sip_tm"].tserver)
        for _, cc := range self.ccmap {
            println(cc.uaA.GetState().String(), cc.uaO.GetState().String())
        }
    //} else {
    //    fmt.Printf("[%d]: %d client, %d server transactions in memory\n",
    //      os.getpid(), len(self.global_config["_sip_tm"].tclient), len(self.global_config["_sip_tm"].tserver))
    }
    if self.safe_restart {
        if len(self.ccmap) == 0 {
            self.sip_tm.Shutdown()
            //os.chdir(self.global_config["_orig_cwd"])
            cmd := exec.Command(os.Args[0], os.Args[1:]...)
            cmd.Env = os.Environ()
            err := cmd.Start()
            if err != nil {
                fmt.Println(err)
                os.Exit(1)
            }
            os.Exit(0)
            // Should not reach this point!
        }
        self.gc_timeout = time.Second
    }
}

/*
    def recvCommand(self, clim, cmd):
        args = cmd.split()
        cmd = args.pop(0).lower()
        if cmd == "q":
            clim.close()
            return false
        if cmd == "l":
            res = "In-memory calls:\n"
            total = 0
            for cc in self.ccmap:
                res += "%s: %s (" % (cc.cId, cc.state.sname)
                if cc.uaA != nil:
                    res += "%s %s:%d %s %s -> " % (cc.uaA.state, cc.uaA.getRAddr0()[0], \
                      cc.uaA.getRAddr0()[1], cc.uaA.getCLD(), cc.uaA.getCLI())
                else:
                    res += "N/A -> "
                if cc.uaO != nil:
                    res += "%s %s:%d %s %s)\n" % (cc.uaO.state, cc.uaO.getRAddr0()[0], \
                      cc.uaO.getRAddr0()[1], cc.uaO.getCLI(), cc.uaO.getCLD())
                else:
                    res += "N/A)\n"
                total += 1
            res += "Total: %d\n" % total
            clim.send(res)
            return false
        if cmd == "lt":
            res = "In-memory server transactions:\n"
            for tid, t in self.global_config["_sip_tm"].tserver.iteritems():
                res += "%s %s %s\n" % (tid, t.method, t.state)
            res += "In-memory client transactions:\n"
            for tid, t in self.global_config["_sip_tm"].tclient.iteritems():
                res += "%s %s %s\n" % (tid, t.method, t.state)
            clim.send(res)
            return false
        if cmd in ("lt", "llt"):
            if cmd == "llt":
                mindur = 60.0
            else:
                mindur = 0.0
            ctime = time()
            res = "In-memory server transactions:\n"
            for tid, t in self.global_config["_sip_tm"].tserver.iteritems():
                duration = ctime - t.rtime
                if duration < mindur:
                    continue
                res += "%s %s %s %s\n" % (tid, t.method, t.state, duration)
            res += "In-memory client transactions:\n"
            for tid, t in self.global_config["_sip_tm"].tclient.iteritems():
                duration = ctime - t.rtime
                if duration < mindur:
                    continue
                res += "%s %s %s %s\n" % (tid, t.method, t.state, duration)
            clim.send(res)
            return false
        if cmd == "d":
            if len(args) != 1:
                clim.send("ERROR: syntax error: d <call-id>\n")
                return false
            if args[0] == "*":
                self.discAll()
                clim.send("OK\n")
                return false
            dlist = [x for x in self.ccmap if str(x.cId) == args[0]]
            if len(dlist) == 0:
                clim.send("ERROR: no call with id of %s has been found\n" % args[0])
                return false
            for cc in dlist:
                cc.disconnect()
            clim.send("OK\n")
            return false
        if cmd == "r":
            if len(args) != 1:
                clim.send("ERROR: syntax error: r [<id>]\n")
                return false
            idx = int(args[0])
            dlist = [x for x in self.ccmap if x.id == idx]
            if len(dlist) == 0:
                clim.send("ERROR: no call with id of %d has been found\n" % idx)
                return false
            for cc in dlist:
                if ! cc.proxied:
                    continue
                if cc.state == CCStateConnected:
                    cc.disconnect(time() - 60)
                    continue
                if cc.state == CCStateARComplete:
                    cc.uaO.disconnect(time() - 60)
                    continue
            clim.send("OK\n")
            return false
        clim.send("ERROR: unknown command\n")
        return false
*/
