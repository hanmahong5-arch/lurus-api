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
import React from 'react';
import { Typography, Tag } from '@douyinfe/semi-ui';
import {
  getLobeHubIcon,
  stringToColor,
  timestamp2string,
} from '../../../helpers';

const { Text } = Typography;

const NAME_RULE_MAP = {
  0: { color: 'green', key: '精确' },
  1: { color: 'blue', key: '前缀' },
  2: { color: 'orange', key: '包含' },
  3: { color: 'purple', key: '后缀' },
};

const InfoRow = ({ label, children }) => (
  <div className='grid grid-cols-[140px_1fr] gap-2 py-2 border-b border-[var(--semi-color-border)]'>
    <div className='text-sm text-[var(--semi-color-text-2)]'>{label}</div>
    <div className='text-sm'>{children}</div>
  </div>
);

const ModelDetailOverview = ({ model, vendorMap, t }) => {
  if (!model) return null;

  const vendor = vendorMap?.[model.vendor_id];
  const ruleConfig = NAME_RULE_MAP[model.name_rule];

  // Parse tags
  const tagsArr = model.tags ? model.tags.split(',').filter(Boolean) : [];

  // Parse endpoints
  let endpointKeys = [];
  try {
    const parsed =
      typeof model.endpoints === 'string'
        ? JSON.parse(model.endpoints)
        : model.endpoints;
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      endpointKeys = Object.keys(parsed);
    } else if (Array.isArray(parsed)) {
      endpointKeys = parsed;
    }
  } catch (_) {
    // ignore parse errors
  }

  return (
    <div>
      <InfoRow label={t('模型名称')}>
        <Text copyable>{model.model_name}</Text>
      </InfoRow>

      <InfoRow label={t('供应商')}>
        {vendor ? (
          <Tag
            color='white'
            shape='circle'
            prefixIcon={getLobeHubIcon(vendor.icon || 'Layers', 14)}
          >
            {vendor.name}
          </Tag>
        ) : (
          '-'
        )}
      </InfoRow>

      <InfoRow label={t('匹配类型')}>
        {ruleConfig ? (
          <Tag size='small' shape='circle' color={ruleConfig.color}>
            {t(ruleConfig.key)}
          </Tag>
        ) : (
          '-'
        )}
      </InfoRow>

      <InfoRow label={t('状态')}>
        <Tag
          size='small'
          shape='circle'
          color={model.status === 1 ? 'green' : 'grey'}
        >
          {model.status === 1 ? t('已启用') : t('已禁用')}
        </Tag>
      </InfoRow>

      <InfoRow label={t('参与官方同步')}>
        <Tag
          size='small'
          shape='circle'
          color={model.sync_official === 1 ? 'green' : 'orange'}
        >
          {model.sync_official === 1 ? t('是') : t('否')}
        </Tag>
      </InfoRow>

      <InfoRow label={t('标签')}>
        {tagsArr.length > 0 ? (
          <div className='flex flex-wrap gap-1'>
            {tagsArr.map((tag, idx) => (
              <Tag
                key={idx}
                size='small'
                shape='circle'
                color={stringToColor(tag)}
              >
                {tag}
              </Tag>
            ))}
          </div>
        ) : (
          '-'
        )}
      </InfoRow>

      <InfoRow label={t('可用分组')}>
        {model.enable_groups?.length > 0 ? (
          <div className='flex flex-wrap gap-1'>
            {model.enable_groups.map((g, idx) => (
              <Tag
                key={idx}
                size='small'
                shape='circle'
                color={stringToColor(g)}
              >
                {g}
              </Tag>
            ))}
          </div>
        ) : (
          '-'
        )}
      </InfoRow>

      <InfoRow label={t('计费类型')}>
        {model.quota_types?.length > 0 ? (
          <div className='flex flex-wrap gap-1'>
            {model.quota_types.map((qt, idx) => {
              if (qt === 0) {
                return (
                  <Tag key={idx} size='small' shape='circle' color='violet'>
                    {t('按量计费')}
                  </Tag>
                );
              }
              if (qt === 1) {
                return (
                  <Tag key={idx} size='small' shape='circle' color='teal'>
                    {t('按次计费')}
                  </Tag>
                );
              }
              return (
                <Tag key={idx} size='small' shape='circle' color='white'>
                  {qt}
                </Tag>
              );
            })}
          </div>
        ) : (
          '-'
        )}
      </InfoRow>

      <InfoRow label={t('端点')}>
        {endpointKeys.length > 0 ? (
          <div className='flex flex-wrap gap-1'>
            {endpointKeys.map((ep, idx) => (
              <Tag
                key={idx}
                size='small'
                shape='circle'
                color={stringToColor(String(ep))}
              >
                {ep}
              </Tag>
            ))}
          </div>
        ) : (
          '-'
        )}
      </InfoRow>

      <InfoRow label={t('描述')}>
        {model.description ? (
          <div className='text-sm whitespace-pre-wrap break-words'>
            {model.description}
          </div>
        ) : (
          '-'
        )}
      </InfoRow>

      <InfoRow label={t('创建时间')}>
        {model.created_time ? timestamp2string(model.created_time) : '-'}
      </InfoRow>

      <InfoRow label={t('更新时间')}>
        {model.updated_time ? timestamp2string(model.updated_time) : '-'}
      </InfoRow>
    </div>
  );
};

export default ModelDetailOverview;
