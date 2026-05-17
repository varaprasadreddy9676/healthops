package monitoring

import (
	"fmt"

	"health-ops/backend/internal/monitoring/cryptoutil"
)

// prepareCheckSecrets transforms inbound plaintext credentials on a check into
// encrypted-at-rest form before persisting. It also honors the SecretMask
// sentinel "********" by preserving the previously stored encrypted value.
//
// existing may be nil for create operations.
func prepareCheckSecrets(check *CheckConfig, existing *CheckConfig) error {
	if check == nil {
		return nil
	}
	// MySQL
	if check.MySQL != nil {
		var prev *MySQLCheckConfig
		if existing != nil {
			prev = existing.MySQL
		}
		if err := prepareMySQLSecrets(check.MySQL, prev); err != nil {
			return err
		}
	}
	// SSH (per-check override)
	if check.SSH != nil {
		var prev *SSHCheckConfig
		if existing != nil {
			prev = existing.SSH
		}
		if err := prepareSSHSecrets(check.SSH, prev); err != nil {
			return err
		}
	}
	return nil
}

func prepareMySQLSecrets(cur *MySQLCheckConfig, prev *MySQLCheckConfig) error {
	if cur == nil {
		return nil
	}
	switch {
	case cur.Password == cryptoutil.SecretMask:
		// UI sent the mask back unchanged — preserve previous encrypted value
		cur.Password = ""
		if prev != nil {
			cur.PasswordEnc = prev.PasswordEnc
			if cur.PasswordEnc == "" && prev.Password != "" {
				// Legacy plaintext stored: lazy-migrate to encrypted-at-rest
				enc, err := cryptoutil.Encrypt(prev.Password)
				if err != nil {
					return fmt.Errorf("encrypt legacy mysql password: %w", err)
				}
				cur.PasswordEnc = enc
			}
		}
	case cur.Password != "":
		// New plaintext supplied — encrypt and drop plaintext
		enc, err := cryptoutil.Encrypt(cur.Password)
		if err != nil {
			return fmt.Errorf("encrypt mysql password: %w", err)
		}
		cur.PasswordEnc = enc
		cur.Password = ""
	case cur.PasswordEnc == "" && prev != nil:
		// No password sent at all; keep previously stored ciphertext on edits
		cur.PasswordEnc = prev.PasswordEnc
	}
	return nil
}

func prepareSSHSecrets(cur *SSHCheckConfig, prev *SSHCheckConfig) error {
	if cur == nil {
		return nil
	}
	switch {
	case cur.Password == cryptoutil.SecretMask:
		cur.Password = ""
		if prev != nil {
			cur.PasswordEnc = prev.PasswordEnc
			if cur.PasswordEnc == "" && prev.Password != "" {
				enc, err := cryptoutil.Encrypt(prev.Password)
				if err != nil {
					return fmt.Errorf("encrypt legacy ssh password: %w", err)
				}
				cur.PasswordEnc = enc
			}
		}
	case cur.Password != "":
		enc, err := cryptoutil.Encrypt(cur.Password)
		if err != nil {
			return fmt.Errorf("encrypt ssh password: %w", err)
		}
		cur.PasswordEnc = enc
		cur.Password = ""
	case cur.PasswordEnc == "" && prev != nil:
		cur.PasswordEnc = prev.PasswordEnc
	}
	return nil
}

// prepareServerSecrets transforms inbound plaintext credentials on a server.
func prepareServerSecrets(srv *RemoteServer, prev *RemoteServer) error {
	if srv == nil {
		return nil
	}
	switch {
	case srv.Password == cryptoutil.SecretMask:
		srv.Password = ""
		if prev != nil {
			srv.PasswordEnc = prev.PasswordEnc
			if srv.PasswordEnc == "" && prev.Password != "" {
				enc, err := cryptoutil.Encrypt(prev.Password)
				if err != nil {
					return fmt.Errorf("encrypt legacy server password: %w", err)
				}
				srv.PasswordEnc = enc
			}
		}
	case srv.Password != "":
		enc, err := cryptoutil.Encrypt(srv.Password)
		if err != nil {
			return fmt.Errorf("encrypt server password: %w", err)
		}
		srv.PasswordEnc = enc
		srv.Password = ""
	case srv.PasswordEnc == "" && prev != nil:
		srv.PasswordEnc = prev.PasswordEnc
	}
	return nil
}
