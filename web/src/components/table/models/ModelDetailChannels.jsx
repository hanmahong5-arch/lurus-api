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
import { Tag, Typography } from '@douyinfe/semi-ui';
import {
  IllustrationNoContent,
  IllustrationNoContentDark,
} from '@douyinfe/semi-illustrations';
import { ExternalLink } from 'lucide-react';
import { stringToColor } from '../../../helpers';

const { Text } = Typography;

const ModelDetailChannels = ({ model, t }) => {
  const channels = model?.bound_channels;

  if (!channels || channels.length === 0) {
    return (
      <div className='flex flex-col items-center justify-center py-10'>
        <IllustrationNoContent style={{ width: 120, height: 120 }} />
        <Text type='tertiary' className='mt-3'>
          {t('暂无绑定渠道')}
        </Text>
      </div>
    );
  }

  return (
    <div className='space-y-2'>
      <div className='text-sm text-[var(--semi-color-text-2)] mb-3'>
        {t('已绑定')} {channels.length} {t('个渠道')}
      </div>
      {channels.map((channel, idx) => (
        <div
          key={idx}
          className='flex items-center justify-between p-3 rounded-lg bg-[var(--semi-color-fill-0)] hover:bg-[var(--semi-color-fill-1)] transition-colors'
        >
          <div className='flex items-center gap-2 min-w-0'>
            <Text className='font-medium truncate'>{channel.name}</Text>
            <Tag
              size='small'
              shape='circle'
              color={stringToColor(String(channel.type))}
            >
              {channel.type}
            </Tag>
          </div>
          <a
            href='/console/channel'
            className='flex items-center gap-1 text-xs text-[var(--semi-color-primary)] hover:underline flex-shrink-0 ml-2'
          >
            {t('查看')}
            <ExternalLink size={12} />
          </a>
        </div>
      ))}
    </div>
  );
};

export default ModelDetailChannels;
