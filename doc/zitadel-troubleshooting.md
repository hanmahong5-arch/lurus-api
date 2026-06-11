# Zitadel Troubleshooting & Fix Guide

> Issue: "User not found" when logging in with admin account.
> Created 2026-01-25 · Last updated 2026-03-06. Namespace: `lurus-identity`.

## Diagnostic Steps

```bash
# 1. Pod status (expect 1/1 Running)
kubectl get pods -n lurus-identity
kubectl describe pod -n lurus-identity <pod>      # if not Running
kubectl logs -n lurus-identity <pod>

# 2. Logs — look for init errors, DB connection issues, "default admin created"
kubectl logs -n lurus-identity -l app=zitadel --tail=100
kubectl logs -n lurus-identity -l app=zitadel -f

# 3. Config
kubectl get deployment -n lurus-identity zitadel -o yaml
kubectl get configmap -n lurus-identity
kubectl get secret -n lurus-identity

# 4. DB connection (env)
kubectl get deployment -n lurus-identity zitadel -o jsonpath='{.spec.template.spec.containers[0].env}' | jq
```

## Solutions

**1. Check default admin account.** Zitadel auto-creates one on first boot. Possible defaults: `admin` / `admin@zitadel.localhost`, or `zitadel-admin@zitadel.localhost`, or a custom FirstInstance account.

```bash
kubectl logs -n lurus-identity -l app=zitadel | grep -i "admin\|password\|created"
kubectl get secret -n lurus-identity -o yaml | grep -i password
```

**2. Reset Zitadel DB (⚠️ test only — deletes all data).** `kubectl scale deployment -n lurus-identity zitadel --replicas=0`, clear its PostgreSQL DB, then `--replicas=1`, watch init logs.

**3. Create admin via CLI.** `kubectl exec -it -n lurus-identity <pod> -- /bin/sh` then run Zitadel CLI (commands version-dependent).

**4. Check version / upgrade.**
```bash
kubectl get deployment -n lurus-identity zitadel -o jsonpath='{.spec.template.spec.containers[0].image}'  # e.g. ghcr.io/zitadel/zitadel:v2.54.0
kubectl set image deployment/zitadel -n lurus-identity zitadel=ghcr.io/zitadel/zitadel:v2.67.1
```

**5. Console first-time setup wizard** at https://auth.lurus.cn, or "Forgot Password" (requires SMTP).

### Re-deploy with FirstInstance (config.yaml ConfigMap)

```yaml
FirstInstance:
  Org:
    Human:
      UserName: admin
      Password: Lurus@ops2026
    Machine:
      UserName: zitadel-admin-sa
      MachineKey:
        ExpirationDate: "2030-01-01T00:00:00Z"
Database:
  postgres:
    Host: <db-host>
    Port: 5432
    Database: zitadel
    User: { Username: zitadel, Password: <password> }
    Admin: { Username: postgres, Password: <admin-password> }
```

(Pair with a `zitadel-admin-sa` Secret holding the service-account JSON.)

## FAQ

- **Q: Can't find admin user?** Likely DB not initialized, init failed, or wrong username/email.
- **Q: Manually create admin?** Yes — via DB, Zitadel CLI, or re-deploy with FirstInstance.
- **Q: Does reset affect other services?** If Zitadel uses a dedicated DB, only auth is affected, but logged-in sessions invalidate.
- **Q: Dump full config?** `kubectl get deployment -n lurus-identity zitadel -o yaml > zitadel-deployment.yaml`.

For escalation, gather: pod status, logs (`--tail=100`), image version, and a screenshot of https://auth.lurus.cn.

## Upgrade History

### 2026-03-06: v2.54.0 → v4.12.0

**Reason**: v2.54.0 JWKS key-rotation bug — on low-traffic instances the signing key expired without auto-rotation, `/oauth/v2/keys` returned empty `{"keys":[]}`, all OIDC logins failed.

**Fix**: upgrade to v4.12.0 (Web Keys feature: signing keys manually managed, no auto-expiry).

**Changes**:
- Image: `ghcr.io/zitadel/zitadel:v2.54.0` → `v4.12.0`
- ConfigMap: `HandleActiveInstances` `87600h` → `720h`
- ConfigMap: `SystemAPIUsers` list format → map format (v4 requirement)
- First boot used `--init-projections` flag to speed projection rebuild

**Verification**: `/oauth/v2/keys` returned 2 valid RSA256 keys; `/debug/healthz` + `/debug/ready` → `ok`; lurus-api log `Successfully refreshed 2 JWKS keys`. DB backup: `/root/zitadel_backup_pre_v4_upgrade.dump` on master.
