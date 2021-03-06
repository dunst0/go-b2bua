// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2015 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2015 Andrii Pylypenko. All rights reserved.
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
package sippy_header

import (
    "sippy/conf"
)

type SipFrom struct {
    compactName
    *sipAddressWithTag
}

var _sip_from_name compactName = newCompactName("From", "f")

func ParseSipFrom(body string, config sippy_conf.Config) ([]SipHeader, error) {
    address, err := ParseSipAddressWithTag(body, config)
    if err != nil {
        return nil, err
    }
    self := &SipFrom{
        compactName       : _sip_from_name,
        sipAddressWithTag : address,
    }
    return []SipHeader{ self }, nil
}

func NewSipFrom(address *sipAddress, config sippy_conf.Config) *SipFrom {
    return &SipFrom{
        compactName       : _sip_from_name,
        sipAddressWithTag : NewSipAddressWithTag(address, config),
    }
}

func (self *SipFrom) Body() string {
    return self.address.String()
}

func (self *SipFrom) String() string {
    return self.LocalStr(nil, false)
}

func (self *SipFrom) LocalStr(hostport *sippy_conf.HostPort, compact bool) string {
    if compact {
        return "f: " + self.Body()
    }
    return "From: " + self.Body()
}

func (self *SipFrom) GetCopy() *SipFrom {
    return &SipFrom{
        compactName       : _sip_from_name,
        sipAddressWithTag : self.sipAddressWithTag.getCopy(),
    }
}

func (self *SipFrom) GetCopyAsIface() SipHeader {
    return self.GetCopy()
}
