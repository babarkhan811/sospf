package lsdb

import (
	"testing"
	"time"

	"github.com/babarkhan811/sospf/internal/packet"
)

func hdr(seq int32, age uint16, sum uint16) packet.LSAHeader {
	return packet.LSAHeader{Type: packet.LSARouter, ID: 1, AdvRouter: 1, SeqNum: seq, Age: age, Checksum: sum}
}

func TestCompareBySeqThenChecksumThenAge(t *testing.T) {
	if Compare(hdr(5, 0, 0), hdr(4, 0, 0)) != Newer {
		t.Error("higher seq should be newer")
	}
	if Compare(hdr(4, 0, 0), hdr(5, 0, 0)) != Older {
		t.Error("lower seq should be older")
	}
	if Compare(hdr(5, 0, 20), hdr(5, 0, 10)) != Newer {
		t.Error("higher checksum breaks a seq tie as newer")
	}
	if Compare(hdr(5, packet.MaxAge, 10), hdr(5, 0, 10)) != Newer {
		t.Error("MaxAge instance should be newer")
	}
	if Compare(hdr(5, 10, 10), hdr(5, 10, 10)) != Same {
		t.Error("identical instances should be Same")
	}
}

func TestInstallOnlyAcceptsNewer(t *testing.T) {
	db := New()
	now := time.Now()
	lsa := &packet.RouterLSA{Header: hdr(packet.InitialSeqNum, 0, 0)}
	lsa.Encode()

	if !db.Install(lsa, now) {
		t.Fatal("first install should change the db")
	}
	if db.Install(lsa, now) {
		t.Fatal("re-installing the same instance should not change the db")
	}

	newer := &packet.RouterLSA{Header: hdr(packet.InitialSeqNum+1, 0, 0)}
	newer.Encode()
	if !db.Install(newer, now) {
		t.Fatal("newer instance should change the db")
	}
	if db.Len() != 1 {
		t.Fatalf("db should hold 1 LSA, has %d", db.Len())
	}

	older := &packet.RouterLSA{Header: hdr(packet.InitialSeqNum, 0, 0)}
	older.Encode()
	if db.Install(older, now) {
		t.Fatal("older instance must be rejected")
	}
}

func TestHasAndAging(t *testing.T) {
	db := New()
	lsa := &packet.RouterLSA{Header: hdr(packet.InitialSeqNum, 0, 0)}
	lsa.Encode()
	db.Install(lsa, time.Now().Add(-10*time.Second))

	if !db.Has(lsa.Header) {
		t.Error("Has should report a stored LSA")
	}
	all := db.All(time.Now())
	if len(all) != 1 {
		t.Fatalf("All returned %d LSAs", len(all))
	}
	if all[0].Header.Age < 9 {
		t.Errorf("age should have advanced ~10s, got %d", all[0].Header.Age)
	}
}
