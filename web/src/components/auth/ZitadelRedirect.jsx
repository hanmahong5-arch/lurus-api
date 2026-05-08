import { useEffect, useState } from 'react';
import Loading from '../common/ui/Loading';
import { Card, Typography } from '@douyinfe/semi-ui';

// register prop kept for backward compat with the route declaration in
// App.jsx; platform identity.lurus.cn renders a unified "登录/注册" UI
// so the same redirect target serves both flows. tenantSlug routing
// dropped — Layer C of ADR-0011 routes login through identity, which
// resolves the account globally rather than per-tenant.
const ZitadelRedirect = (_props) => {
  const [showFallback, setShowFallback] = useState(false);

  useEffect(() => {
    // After identity finishes the login dance the browser comes back
    // to the v2 dashboard with the lurus_session cookie set on the
    // parent .lurus.cn domain.
    const returnTo = `${window.location.origin}/console/v2/dashboard`;
    const url = `/api/v2/auth/zita-login?return_to=${encodeURIComponent(returnTo)}`;

    const timer = setTimeout(() => setShowFallback(true), 3000);
    window.location.href = url;
    return () => clearTimeout(timer);
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
