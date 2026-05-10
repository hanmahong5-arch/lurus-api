import { useEffect, useState } from 'react';
import Loading from '../common/ui/Loading';
import { Card, Typography } from '@douyinfe/semi-ui';
import { API } from '../../helpers';

// register prop kept for backward compat with the route declaration in
// App.jsx; platform identity.lurus.cn renders a unified "登录/注册" UI
// so the same redirect target serves both flows. tenantSlug routing
// dropped — Layer C of ADR-0011 routes login through identity, which
// resolves the account globally rather than per-tenant.
const ZitadelRedirect = (_props) => {
  const [showFallback, setShowFallback] = useState(false);

  useEffect(() => {
    let cancelled = false;
    let timer;

    // Probe existing platform session via SDK middleware before redirecting
    // to identity. If the cookie is valid we are already authenticated —
    // synthesize a minimum localStorage 'user' to satisfy PrivateRoute and
    // navigate to dashboard. Without this bootstrap, after identity bounces
    // back PrivateRoute kicks to /login → ZitadelRedirect remounts →
    // window.location to identity → identity bounces (still has session) →
    // PrivateRoute again → infinite full-page loop.
    //
    // Layer C cleanup will replace the synthesized user with a proper
    // newhub user lookup by lurus_account_id + Bearer token issuance.
    const bootstrap = async () => {
      try {
        const res = await API.get('/api/v2/me/zita', {
          skipErrorHandler: true,
        });
        if (cancelled) return;
        if (res?.data?.account_id) {
          const synthUser = {
            id: res.data.account_id,
            username: 'zita_session_' + res.data.account_id,
            role: 1,
            zita_session: true,
          };
          localStorage.setItem('user', JSON.stringify(synthUser));
          window.location.replace(
            window.location.origin + '/console/v2/dashboard'
          );
          return;
        }
      } catch (_) {
        // 401 / network — fall through to identity redirect.
      }
      if (cancelled) return;

      const returnTo = `${window.location.origin}/console/v2/dashboard`;
      const url = `/api/v2/auth/zita-login?return_to=${encodeURIComponent(returnTo)}`;
      timer = setTimeout(() => setShowFallback(true), 3000);
      window.location.href = url;
    };

    bootstrap();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, []);

  return (
    <div className='flex flex-col items-center justify-center min-h-screen bg-gray-50'>
      <Loading />
      {showFallback && (
        <div className='mt-8'>
          <Card className='p-6 shadow-lg'>
            <Typography.Text className='text-gray-600 block text-center'>
              正在跳转到统一登录，请稍候...
            </Typography.Text>
          </Card>
        </div>
      )}
    </div>
  );
};

export default ZitadelRedirect;
