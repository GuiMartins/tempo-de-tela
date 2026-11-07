package store

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/inventory"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/oui"
)

type Observation struct {
	MAC                               net.HardwareAddr
	IP                                netip.Addr
	Hostname, DHCPVendor, Fingerprint string
}

func (s *Store) ObserveDevice(ctx context.Context, o Observation, seen time.Time, lifetime time.Duration) error {
	if len(o.MAC) != 6 || o.MAC[0]&1 != 0 || bytes.Equal(o.MAC, make([]byte, 6)) {
		return fmt.Errorf("MAC inválido: %s", o.MAC)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ts := seen.Unix()
	mac := o.MAC.String()
	var id int64
	var clues inventory.Clues
	err = tx.QueryRowContext(ctx, `
		INSERT INTO devices (key, mac, vendor, random_mac, hostname, dhcp_vendor, dhcp_fingerprint, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			hostname = coalesce(nullif(excluded.hostname, ''), hostname),
			dhcp_vendor = coalesce(nullif(excluded.dhcp_vendor, ''), dhcp_vendor),
			dhcp_fingerprint = coalesce(nullif(excluded.dhcp_fingerprint, ''), dhcp_fingerprint),
			first_seen = min(first_seen, excluded.first_seen),
			last_seen = max(last_seen, excluded.last_seen)
		RETURNING id, hostname, mdns_name, vendor, dhcp_vendor, dhcp_fingerprint, random_mac`,
		"mac:"+mac, mac, oui.Vendor(o.MAC), oui.IsRandomized(o.MAC), o.Hostname, o.DHCPVendor, o.Fingerprint, ts, ts).
		Scan(&id, &clues.Hostname, &clues.MDNSName, &clues.Vendor, &clues.DHCPVendor, &clues.Fingerprint, &clues.RandomMAC)
	if err != nil {
		return err
	}
	os, kind := inventory.Guess(clues)
	if _, err := tx.ExecContext(ctx, `UPDATE devices SET os_guess = ?, kind_guess = ? WHERE id = ?`, os, kind, id); err != nil {
		return err
	}
	if ip := o.IP.Unmap().WithZone(""); ip.IsValid() && !ip.IsUnspecified() {
		if err := bindAddress(ctx, tx, ip, id, seen.UnixNano(), seen.Add(lifetime).UnixNano()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func bindAddress(ctx context.Context, tx *sql.Tx, ip netip.Addr, id, since, until int64) error {
	addr := ip.String()
	var previousID, previousSince, previousUntil int64
	err := tx.QueryRowContext(ctx, `
		SELECT device_id, since, valid_until FROM device_addresses WHERE ip = ? ORDER BY since DESC LIMIT 1`, addr).
		Scan(&previousID, &previousSince, &previousUntil)
	if err == nil {
		if since < previousSince {
			return nil
		}
		if previousID == id && since <= previousUntil {
			_, err := tx.ExecContext(ctx, `UPDATE device_addresses SET valid_until = max(valid_until, ?) WHERE ip = ? AND since = ?`,
				until, addr, previousSince)
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE device_addresses SET valid_until = min(valid_until, ?) WHERE ip = ? AND since = ?`,
			since, addr, previousSince); err != nil {
			return err
		}
	} else if err != sql.ErrNoRows {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO device_addresses(ip, since, valid_until, device_id) VALUES (?, ?, ?, ?)
		ON CONFLICT(ip, since) DO UPDATE SET valid_until = excluded.valid_until, device_id = excluded.device_id`,
		addr, since, until, id); err != nil {
		return err
	}

	column := "ipv6"
	if ip.Is4() {
		column = "ipv4"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE devices SET `+column+` = '' WHERE `+column+` = ? AND mac <> '' AND id <> ?`, addr, id); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE devices SET `+column+` = ? WHERE id = ?`, addr, id)
	return err
}

func deviceAt(ctx context.Context, tx *sql.Tx, ip string, ts int64) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		SELECT device_id FROM device_addresses WHERE ip = ? AND since <= ? AND valid_until > ?
		ORDER BY since DESC LIMIT 1`, ip, ts, ts).Scan(&id)
	return id, err
}

func (s *Store) ObserveMDNS(ctx context.Context, ip netip.Addr, name string, seen time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id, err := deviceAt(ctx, tx, ip.Unmap().WithZone("").String(), seen.UnixNano())
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var clues inventory.Clues
	err = tx.QueryRowContext(ctx, `
		UPDATE devices SET mdns_name = ? WHERE id = ?
		RETURNING hostname, mdns_name, vendor, dhcp_vendor, dhcp_fingerprint, random_mac`, name, id).
		Scan(&clues.Hostname, &clues.MDNSName, &clues.Vendor, &clues.DHCPVendor, &clues.Fingerprint, &clues.RandomMAC)
	if err != nil {
		return err
	}
	os, kind := inventory.Guess(clues)
	if _, err := tx.ExecContext(ctx, `UPDATE devices SET os_guess = ?, kind_guess = ? WHERE id = ?`, os, kind, id); err != nil {
		return err
	}
	return tx.Commit()
}
