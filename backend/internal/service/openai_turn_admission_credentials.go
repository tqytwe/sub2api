package service

import "reflect"

// bindOpenAITurnCredentialParent keeps authoritative credential state request-local.
// Copy the account rather than mutating repository/scheduler-owned state. The
// marker lets token acquisition distinguish this snapshot from stale scheduler
// data, for which the ordinary token-provider cache remains useful.
func bindOpenAITurnCredentialParent(account, parent *Account) *Account {
	if account == nil {
		return nil
	}
	if account.IsShadow() && (parent == nil || parent.ID != *account.ParentAccountID) {
		return account
	}
	admitted := snapshotOpenAITurnCredentialAccount(account)
	if admitted == nil {
		return nil
	}
	admitted.openAITurnCredentialsAdmitted = true
	admitted.openAITurnCredentialForShadow = false
	admitted.openAITurnCredentialParent = nil
	if account.IsShadow() {
		admittedParent := snapshotOpenAITurnCredentialAccount(parent)
		if admittedParent == nil {
			return nil
		}
		admittedParent.openAITurnCredentialsAdmitted = true
		admittedParent.openAITurnCredentialForShadow = true
		admitted.openAITurnCredentialParent = admittedParent
	}
	return admitted
}

// A secret refresh can proceed on the same credential parent, but changing its
// upstream identity or transport after request preparation requires a new turn.
func validateOpenAITurnCredentialParentBinding(selected, parent *Account) error {
	if selected == nil || selected.openAITurnCredentialParent == nil {
		return nil
	}
	previous := selected.openAITurnCredentialParent
	if parent == nil || parent.ID != previous.ID || openAITurnRouteFingerprint(parent) != openAITurnRouteFingerprint(previous) {
		return denyOpenAITurn("credential_parent_binding_changed")
	}
	return nil
}

// Snapshot only the request's account and one credential parent, never their
// repository entity graph. Detach route/auth data without JSON round-tripping:
// numeric values and typed maps/slices retain their existing getter semantics.
func snapshotOpenAITurnCredentialAccount(account *Account) *Account {
	if account == nil {
		return nil
	}
	remaining := 1 << 16
	credentials, ok := cloneOpenAITurnCredentialValue(reflect.ValueOf(account.Credentials), 0, &remaining)
	if !ok {
		return nil
	}
	extra, ok := cloneOpenAITurnCredentialValue(reflect.ValueOf(account.Extra), 0, &remaining)
	if !ok {
		return nil
	}
	snapshot := *account
	snapshot.Credentials, ok = credentials.Interface().(map[string]any)
	if !ok {
		return nil
	}
	snapshot.Extra, ok = extra.Interface().(map[string]any)
	if !ok {
		return nil
	}
	snapshot.ParentAccountID = cloneAccountValuePointer(account.ParentAccountID)
	snapshot.ProxyID = cloneAccountValuePointer(account.ProxyID)
	snapshot.ProxyFallbackOriginID = cloneAccountValuePointer(account.ProxyFallbackOriginID)
	snapshot.ExpiresAt = cloneAccountValuePointer(account.ExpiresAt)
	snapshot.RateLimitResetAt = cloneAccountValuePointer(account.RateLimitResetAt)
	snapshot.OverloadUntil = cloneAccountValuePointer(account.OverloadUntil)
	snapshot.TempUnschedulableUntil = cloneAccountValuePointer(account.TempUnschedulableUntil)
	if account.Proxy != nil {
		proxy := *account.Proxy
		proxy.ExpiresAt = cloneAccountValuePointer(account.Proxy.ExpiresAt)
		proxy.BackupProxyID = cloneAccountValuePointer(account.Proxy.BackupProxyID)
		snapshot.Proxy = &proxy
	}
	// Rebuild derived caches from the detached credential document on demand.
	snapshot.modelMappingCache, snapshot.modelMappingCacheReady = nil, false
	snapshot.headerOverrideCache, snapshot.headerOverrideCacheReady = nil, false
	return &snapshot
}

// Persisted credential/extra documents contain JSON values; support their typed
// in-memory map/slice equivalents as well. Bound cycles and oversized/deep
// documents rather than returning an aliased or partially copied snapshot.
func cloneOpenAITurnCredentialValue(value reflect.Value, depth int, remaining *int) (reflect.Value, bool) {
	if depth > 64 || *remaining <= 0 {
		return reflect.Value{}, false
	}
	*remaining = *remaining - 1
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type()), true
		}
		child, ok := cloneOpenAITurnCredentialValue(value.Elem(), depth+1, remaining)
		if !ok {
			return reflect.Value{}, false
		}
		if value.Kind() == reflect.Pointer {
			out := reflect.New(value.Type().Elem())
			out.Elem().Set(child)
			return out, true
		}
		out := reflect.New(value.Type()).Elem()
		out.Set(child)
		return out, true
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String || value.Len() > *remaining {
			return reflect.Value{}, false
		}
		if value.IsNil() {
			return reflect.Zero(value.Type()), true
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			child, ok := cloneOpenAITurnCredentialValue(iter.Value(), depth+1, remaining)
			if !ok {
				return reflect.Value{}, false
			}
			out.SetMapIndex(iter.Key(), child)
		}
		return out, true
	case reflect.Slice, reflect.Array:
		if value.Len() > *remaining {
			return reflect.Value{}, false
		}
		var out reflect.Value
		if value.Kind() == reflect.Slice {
			if value.IsNil() {
				return reflect.Zero(value.Type()), true
			}
			out = reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		} else {
			out = reflect.New(value.Type()).Elem()
		}
		for i := 0; i < value.Len(); i++ {
			child, ok := cloneOpenAITurnCredentialValue(value.Index(i), depth+1, remaining)
			if !ok {
				return reflect.Value{}, false
			}
			out.Index(i).Set(child)
		}
		return out, true
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return value, true
	default:
		return reflect.Value{}, false
	}
}
