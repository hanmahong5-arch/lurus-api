/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import React, { useState } from 'react';
import { SideSheet, Tabs, Button, Space, Modal, Tag } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { getLobeHubIcon } from '../../../helpers/render';
import ModelDetailOverview from './ModelDetailOverview';
import ModelDetailPricing from './ModelDetailPricing';
import ModelDetailChannels from './ModelDetailChannels';

const ModelDetailDrawer = ({
  visible,
  model,
  onClose,
  pricingMap,
  vendorMap,
  manageModel,
  setEditingModel,
  setShowEdit,
  refresh,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [activeTab, setActiveTab] = useState('overview');

  if (!model) return null;

  const vendor = vendorMap?.[model.vendor_id];
  const iconKey = model.icon || vendor?.icon;
  const isEnabled = model.status === 1;

  const handleToggleStatus = () => {
    const action = isEnabled ? 'disable' : 'enable';
    manageModel(model.id, action, model);
  };

  const handleEdit = () => {
    setEditingModel(model);
    setShowEdit(true);
    onClose();
  };

  const handleDelete = () => {
    Modal.confirm({
      title: t('确定是否要删除此模型？'),
      content: t('此修改将不可逆'),
      onOk: async () => {
        await manageModel(model.id, 'delete', model);
        await refresh();
        onClose();
      },
    });
  };

  const headerContent = (
    <div className='flex items-center gap-3 min-w-0'>
      <div className='flex-shrink-0'>{getLobeHubIcon(iconKey, 28)}</div>
      <div className='min-w-0 flex-1'>
        <div className='flex items-center gap-2 flex-wrap'>
          <span className='text-base font-semibold truncate'>
            {model.model_name}
          </span>
          <Tag size='small' shape='circle' color={isEnabled ? 'green' : 'grey'}>
            {isEnabled ? t('已启用') : t('已禁用')}
          </Tag>
        </div>
        {vendor && (
          <div className='text-xs text-[var(--semi-color-text-2)] mt-0.5'>
            {vendor.name}
          </div>
        )}
      </div>
    </div>
  );

  return (
    <SideSheet
      placement='right'
      title={headerContent}
      visible={visible}
      width={isMobile ? '100%' : 640}
      onCancel={onClose}
      bodyStyle={{
        padding: 0,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}
    >
      <div className='flex flex-col h-full'>
        <div className='flex-1 overflow-auto px-4'>
          <Tabs type='line' activeKey={activeTab} onChange={setActiveTab}>
            <Tabs.TabPane tab={t('概览')} itemKey='overview'>
              <div className='py-3'>
                <ModelDetailOverview
                  model={model}
                  vendorMap={vendorMap}
                  t={t}
                />
              </div>
            </Tabs.TabPane>
            <Tabs.TabPane tab={t('定价')} itemKey='pricing'>
              <div className='py-3'>
                <ModelDetailPricing
                  model={model}
                  pricingMap={pricingMap}
                  t={t}
                />
              </div>
            </Tabs.TabPane>
            <Tabs.TabPane tab={t('渠道')} itemKey='channels'>
              <div className='py-3'>
                <ModelDetailChannels model={model} t={t} />
              </div>
            </Tabs.TabPane>
          </Tabs>
        </div>

        <div className='flex-shrink-0 border-t border-[var(--semi-color-border)] px-4 py-3'>
          <Space>
            <Button
              type={isEnabled ? 'danger' : 'primary'}
              size='small'
              onClick={handleToggleStatus}
            >
              {isEnabled ? t('禁用') : t('启用')}
            </Button>
            <Button type='tertiary' size='small' onClick={handleEdit}>
              {t('编辑')}
            </Button>
            <Button type='danger' size='small' onClick={handleDelete}>
              {t('删除')}
            </Button>
          </Space>
        </div>
      </div>
    </SideSheet>
  );
};

export default ModelDetailDrawer;
