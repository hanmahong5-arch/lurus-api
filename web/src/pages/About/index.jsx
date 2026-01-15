/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import React, { useEffect, useState, useMemo } from 'react';
import { Card, Typography, Button, Space, Spin, Tag } from '@douyinfe/semi-ui';
import { IconDownload, IconChevronDown } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Title, Text, Paragraph } = Typography;

const detectOS = () => {
  const ua = navigator.userAgent.toLowerCase();
  if (ua.includes('win')) return 'windows';
  if (ua.includes('mac')) return 'darwin';
  if (ua.includes('linux')) return 'linux';
  return 'windows';
};

const platformInfo = {
  windows: { name: 'Windows', icon: '🪟' },
  darwin: { name: 'macOS', icon: '🍎' },
  linux: { name: 'Linux', icon: '🐧' },
};

const typeLabels = {
  installer: { zh: '安装版', en: 'Installer' },
  portable: { zh: '便携版', en: 'Portable' },
  dmg: { zh: 'DMG', en: 'DMG' },
  zip: { zh: 'ZIP', en: 'ZIP' },
  appimage: { zh: 'AppImage', en: 'AppImage' },
  deb: { zh: 'DEB', en: 'DEB' },
};

const iconMap = { codeswitch: '🔄', gemini: '💎', 'gemini-gui': '💎' };

const formatSize = (bytes) => {
  if (!bytes) return '';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  while (bytes >= 1024 && i < 3) { bytes /= 1024; i++; }
  return bytes.toFixed(1) + ' ' + units[i];
};

const ProductCard = ({ product, release, currentOS, lang, t }) => {
  const [showAll, setShowAll] = useState(false);
  const getLocalized = (obj) => obj?.[lang] || obj?.en || obj?.zh || (typeof obj === 'string' ? obj : '');
  const recommended = release?.assets?.filter((a) => a.platform === currentOS) || [];
  const others = release?.assets?.filter((a) => a.platform !== currentOS) || [];
  const download = (a) => window.open('/downloads/' + a.downloadUrl, '_blank');
  const label = (a) => { const t = typeLabels[a.type]?.[lang] || a.type; const s = formatSize(a.size); return s ? t+' ('+s+')' : t; };

  return (
    <Card className='w-full max-w-md' headerStyle={{ padding: '16px 20px' }} bodyStyle={{ padding: '20px' }}
      title={<div className='flex items-center gap-2'>
        <span className='text-2xl'>{iconMap[product.icon] || '📦'}</span>
        <span>{product.displayName || product.name}</span>
        {release?.version && <Tag color='green' size='small'>v{release.version}</Tag>}
      </div>}>
      <Paragraph className='mb-4 text-semi-color-text-2'>{getLocalized(product.description)}</Paragraph>
      {product.features?.length > 0 && (
        <div className='mb-4 flex flex-wrap gap-2'>
          {product.features.slice(0, 3).map((f, i) => <Tag key={i} color='light-blue' size='small'>{getLocalized(f.title)}</Tag>)}
        </div>
      )}
      {recommended.length > 0 && (
        <div className='mb-4'>
          <Text strong className='block mb-2'>{t('推荐下载')} ({platformInfo[currentOS]?.name})</Text>
          <Space vertical align='start' className='w-full'>
            {recommended.map((a, i) => <Button key={i} theme='solid' type='primary' size='large' icon={<IconDownload />} className='w-full' onClick={() => download(a)}>{label(a)}</Button>)}
          </Space>
        </div>
      )}
      {others.length > 0 && (
        <div>
          <Button type='tertiary' size='small' icon={<IconChevronDown />} iconPosition='right' onClick={() => setShowAll(!showAll)} className='mb-2'>{t('其他平台')} ({others.length})</Button>
          {showAll && <Space wrap className='mt-2'>{others.map((a, i) => <Button key={i} theme='light' icon={<IconDownload />} onClick={() => download(a)}>{platformInfo[a.platform]?.icon} {label(a)}</Button>)}</Space>}
        </div>
      )}
      {release?.changelog && (
        <div className='mt-4 pt-4 border-t border-semi-color-border'>
          <Text type='tertiary' size='small'>{t('更新日志')}:</Text>
          <ul className='mt-1 text-sm text-semi-color-text-2 list-disc list-inside'>
            {(getLocalized(release.changelog) || []).slice(0, 2).map((c, i) => <li key={i}>{c}</li>)}
          </ul>
        </div>
      )}
    </Card>
  );
};

const QuickStartGuide = ({ t }) => (
  <Card className='w-full max-w-2xl mt-8'>
    <Title heading={4} className='mb-4'>{t('快速开始')}</Title>
    <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
      {[{ n: '1', t: t('下载对应平台的安装包') }, { n: '2', t: t('安装并启动应用') }, { n: '3', t: t('配置API Token') }, { n: '4', t: t('开始使用') }].map((s) => (
        <div key={s.n} className='flex items-center gap-3 p-3 rounded-lg bg-semi-color-fill-0'>
          <div className='w-8 h-8 rounded-full bg-semi-color-primary flex items-center justify-center text-white font-bold'>{s.n}</div>
          <Text>{s.t}</Text>
        </div>
      ))}
    </div>
  </Card>
);

const Download = () => {
  const { t, i18n } = useTranslation();
  const [products, setProducts] = useState([]);
  const [loading, setLoading] = useState(true);
  const currentOS = useMemo(() => detectOS(), []);
  const lang = i18n.language?.startsWith('zh') ? 'zh' : 'en';

  useEffect(() => {
    (async () => {
      try {
        const cfg = await (await fetch('/downloads/config/products.json')).json();
        const loaded = await Promise.all(cfg.products.filter((p) => p.enabled).sort((a, b) => a.order - b.order).map(async (p) => {
          const [prod, rel] = await Promise.all([fetch('/downloads/products/' + p.id + '/product.json'), fetch('/downloads/products/' + p.id + '/releases/latest.json')]);
          return { ...(await prod.json()), release: await rel.json(), featured: p.featured };
        }));
        setProducts(loaded);
      } catch (e) { console.error(e); }
      setLoading(false);
    })();
  }, []);

  if (loading) return <div className='mt-[60px] flex justify-center items-center min-h-[50vh]'><Spin size='large' /></div>;

  return (
    <div className='mt-[60px] px-4 py-8'>
      <div className='text-center mb-8'>
        <Title heading={1} className='mb-2'>{t('下载中心')}</Title>
        <Text type='secondary' size='large'>{t('选择您的平台')}</Text>
      </div>
      <div className='flex flex-wrap justify-center gap-6'>
        {products.map((p) => <ProductCard key={p.id} product={p} release={p.release} currentOS={currentOS} lang={lang} t={t} />)}
      </div>
      <div className='flex justify-center'><QuickStartGuide t={t} /></div>
    </div>
  );
};

export default Download;
