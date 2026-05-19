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

// Page-level fixed action rail for the Channel management page.
// Acts on the current selection (checkboxes on channel rows + checkboxes on
// model rows inside expanded sub-tables). Single-row-only actions are grayed
// out under multi-select or mixed selection. The rail is sticky to the
// viewport's top and does not move with the table's vertical or horizontal
// scroll.

import React, { useMemo } from 'react';
import { Button, Modal, Tooltip, Divider } from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconRefresh,
  IconEdit,
  IconCopy,
  IconDelete,
  IconKey,
  IconList,
  IconLayers,
  IconSetting,
} from '@douyinfe/semi-icons';
import {
  FaBolt,
  FaPlay,
  FaStop,
  FaServer,
  FaCheckDouble,
} from 'react-icons/fa';

const RailButton = ({ tooltip, icon, onClick, disabled, danger }) => (
  <Tooltip content={tooltip} position='right'>
    <Button
      size='large'
      theme={danger && !disabled ? 'light' : 'borderless'}
      type={danger ? 'danger' : 'tertiary'}
      icon={icon}
      onClick={onClick}
      disabled={disabled}
      style={{ width: 40, height: 40 }}
      aria-label={tooltip}
    />
  </Tooltip>
);

const RailDivider = () => <Divider style={{ width: 28, margin: '4px 0' }} />;

const ChannelsActionRail = (props) => {
  const {
    t,
    selectedChannels = [],
    selectedModels = [],
    setSelectedModels,
    refresh,
    setEditingChannel,
    setShowEdit,
    copySelectedChannel,
    manageChannel,
    batchDeleteChannels,
    testChannel,
    testAllChannels,
    checkOllamaVersion,
    removeModelsFromChannel,
    setCurrentMultiKeyChannel,
    setShowMultiKeyManageModal,
    setShowColumnSelector,
    compactMode,
    setCompactMode,
  } = props;

  const channelCount = selectedChannels.length;
  const modelCount = selectedModels.length;
  const total = channelCount + modelCount;
  const mixed = channelCount > 0 && modelCount > 0;

  // Mode of the rail determines which handler an icon dispatches to.
  // - 'channel': channels selected, no models
  // - 'model': models selected, no channels
  // - 'mixed': both — single-row icons disabled
  // - 'none': nothing selected
  const mode = mixed
    ? 'mixed'
    : channelCount > 0
      ? 'channel'
      : modelCount > 0
        ? 'model'
        : 'none';

  const singleChannel =
    mode === 'channel' && channelCount === 1 ? selectedChannels[0] : null;
  const singleModel =
    mode === 'model' && modelCount === 1 ? selectedModels[0] : null;

  const isTagRow = singleChannel?.children !== undefined;
  const isMultiKey = !!singleChannel?.channel_info?.is_multi_key;
  const isOllama = singleChannel?.type === 4;

  const enableSingleChannel =
    mode === 'channel' && channelCount === 1 && !isTagRow;
  const enableSingleModel = mode === 'model' && modelCount === 1;

  const allChannelsDisabled = useMemo(
    () =>
      selectedChannels.length > 0 &&
      selectedChannels.every((c) => c.status !== 1),
    [selectedChannels],
  );

  // Action handlers --------------------------------------------------------

  const handleNewChannel = () => {
    setEditingChannel({ id: undefined });
    setShowEdit(true);
  };

  // 新增模型：直接在所选渠道上加。模型行选中时，目标渠道 = 该模型的父渠道。
  const handleNewModel = () => {
    let target = null;
    if (enableSingleChannel) target = singleChannel;
    else if (enableSingleModel) target = singleModel.channel;
    if (!target) {
      Modal.info({
        title: t('请先选中目标渠道'),
        content: t('勾选 1 个渠道（或勾选 1 个模型行），然后再点此图标。'),
      });
      return;
    }
    setEditingChannel(target);
    setShowEdit(true);
  };

  const handleEdit = () => {
    if (mode === 'channel' && enableSingleChannel) {
      setEditingChannel(singleChannel);
      setShowEdit(true);
    } else if (mode === 'model' && enableSingleModel) {
      // Model edit currently routes to the parent channel's full editor —
      // user adjusts mapping there. Finer-grained per-model modal can be added later.
      setEditingChannel(singleModel.channel);
      setShowEdit(true);
    }
  };

  const handleCopy = () => {
    if (!enableSingleChannel) return;
    Modal.confirm({
      title: t('确定是否要复制此渠道？'),
      content: t('复制渠道的所有信息'),
      onOk: () => copySelectedChannel(singleChannel),
    });
  };

  const handleDelete = () => {
    if (mode === 'channel' && channelCount > 0) {
      Modal.confirm({
        title: t('确定是否要删除所选通道？'),
        content: t('此修改将不可逆'),
        onOk: () => batchDeleteChannels(),
      });
      return;
    }
    if (mode === 'model' && modelCount > 0) {
      // Group model deletions by channel — one PUT per channel.
      const byChannel = new Map();
      selectedModels.forEach((m) => {
        const list = byChannel.get(m.channelId) || [];
        list.push(m.modelName);
        byChannel.set(m.channelId, list);
      });
      Modal.confirm({
        title: t('从 ${c} 个渠道移除 ${m} 个模型？')
          .replace('${c}', byChannel.size)
          .replace('${m}', modelCount),
        content: t('该操作仅修改渠道的模型列表，不影响其它渠道'),
        onOk: async () => {
          for (const [channelId, names] of byChannel.entries()) {
            await removeModelsFromChannel(channelId, names);
          }
          if (setSelectedModels) setSelectedModels([]);
        },
      });
    }
  };

  const handleTest = () => {
    if (mode === 'channel' && channelCount > 0) {
      selectedChannels.forEach((ch) => {
        if (ch.children !== undefined) return;
        testChannel(ch, '');
      });
      return;
    }
    if (mode === 'model' && modelCount > 0) {
      selectedModels.forEach((m) => {
        testChannel(m.channel, m.modelName);
      });
    }
  };

  const handleToggleStatus = () => {
    if (mode !== 'channel' || channelCount === 0) return;
    const target = allChannelsDisabled ? 'enable' : 'disable';
    selectedChannels.forEach((ch) => {
      if (ch.children !== undefined) return;
      manageChannel(ch.id, target, ch);
    });
  };

  const handleMultiKey = () => {
    if (!enableSingleChannel || !isMultiKey) return;
    setCurrentMultiKeyChannel(singleChannel);
    setShowMultiKeyManageModal(true);
  };

  const handleOllamaCheck = () => {
    if (!enableSingleChannel || !isOllama) return;
    checkOllamaVersion(singleChannel);
  };

  const handleTestAll = () => {
    Modal.confirm({
      title: t('确定？'),
      content: t('确定要测试所有通道吗？'),
      onOk: () => testAllChannels(),
    });
  };

  // Disabled rules ---------------------------------------------------------
  const dis = {
    newModel: !(enableSingleChannel || enableSingleModel),
    test: total === 0 || mixed || isTagRow,
    toggle: mode !== 'channel' || channelCount === 0 || isTagRow,
    edit: !(enableSingleChannel || enableSingleModel),
    copy: !enableSingleChannel,
    del: total === 0 || mixed,
    multiKey: !enableSingleChannel || !isMultiKey,
    ollama: !enableSingleChannel || !isOllama,
  };

  const tipFor = (key) => {
    if (mixed) return t('混合选中渠道+模型时不可用');
    return null;
  };

  const railStyle = {
    position: 'sticky',
    top: 0,
    alignSelf: 'flex-start',
    width: 56,
    minWidth: 56,
    height: 'calc(100vh - 56px)',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 4,
    padding: '8px 0',
    background: 'var(--semi-color-bg-1)',
    border: '1px solid var(--semi-color-border)',
    borderRadius: 12,
    overflowY: 'auto',
    overflowX: 'hidden',
    zIndex: 5,
  };

  return (
    <div style={railStyle} className='channels-action-rail'>
      <RailButton
        tooltip={t('新增渠道')}
        icon={<IconPlus />}
        onClick={handleNewChannel}
      />
      <RailButton
        tooltip={
          dis.newModel
            ? t('新增模型（请先选择目标渠道或模型）')
            : t('在选中渠道下新增模型')
        }
        icon={<IconLayers />}
        onClick={handleNewModel}
        disabled={dis.newModel}
      />
      <RailButton
        tooltip={t('刷新')}
        icon={<IconRefresh />}
        onClick={() => refresh()}
      />

      <RailDivider />

      <RailButton
        tooltip={
          tipFor('test') ||
          (total === 0
            ? t('测试（请先选择）')
            : mode === 'channel'
              ? channelCount === 1
                ? t('测试该渠道')
                : t('测试选中的 ${n} 个渠道').replace('${n}', channelCount)
              : modelCount === 1
                ? t('测试该模型')
                : t('测试选中的 ${n} 个模型').replace('${n}', modelCount))
        }
        icon={<FaBolt />}
        onClick={handleTest}
        disabled={dis.test}
      />
      <RailButton
        tooltip={
          tipFor('toggle') ||
          (mode !== 'channel'
            ? t('启用/禁用（仅渠道行可用）')
            : channelCount === 0
              ? t('启用/禁用（请先选择渠道）')
              : allChannelsDisabled
                ? t('启用选中')
                : t('禁用选中'))
        }
        icon={allChannelsDisabled ? <FaPlay /> : <FaStop />}
        onClick={handleToggleStatus}
        disabled={dis.toggle}
      />
      <RailButton
        tooltip={
          tipFor('edit') ||
          (enableSingleChannel
            ? t('编辑该渠道')
            : enableSingleModel
              ? t('编辑该模型（在父渠道编辑器中）')
              : total > 1
                ? t('编辑（仅支持单选）')
                : t('编辑（请先选择）'))
        }
        icon={<IconEdit />}
        onClick={handleEdit}
        disabled={dis.edit}
      />
      <RailButton
        tooltip={
          tipFor('copy') ||
          (enableSingleChannel
            ? t('复制该渠道')
            : channelCount > 1
              ? t('复制（仅支持单选）')
              : t('复制（仅渠道行可用，需单选）'))
        }
        icon={<IconCopy />}
        onClick={handleCopy}
        disabled={dis.copy}
      />
      <RailButton
        tooltip={
          tipFor('del') ||
          (total === 0
            ? t('删除（请先选择）')
            : mode === 'channel'
              ? t('删除选中的 ${n} 个渠道').replace('${n}', channelCount)
              : t('从渠道移除选中的 ${n} 个模型').replace('${n}', modelCount))
        }
        icon={<IconDelete />}
        onClick={handleDelete}
        disabled={dis.del}
        danger
      />

      <RailDivider />

      <RailButton
        tooltip={
          enableSingleChannel && isMultiKey
            ? t('多密钥管理')
            : t('多密钥管理（仅多密钥渠道行可用）')
        }
        icon={<IconKey />}
        onClick={handleMultiKey}
        disabled={dis.multiKey}
      />
      <RailButton
        tooltip={
          enableSingleChannel && isOllama
            ? t('Ollama 测活')
            : t('Ollama 测活（仅 Ollama 渠道可用）')
        }
        icon={<FaServer />}
        onClick={handleOllamaCheck}
        disabled={dis.ollama}
      />

      <RailDivider />

      <RailButton
        tooltip={t('批量测试所有已启用渠道')}
        icon={<FaCheckDouble />}
        onClick={handleTestAll}
      />
      <RailButton
        tooltip={compactMode ? t('切换到宽松列表') : t('切换到紧凑列表')}
        icon={<IconList />}
        onClick={() => setCompactMode(!compactMode)}
      />
      <RailButton
        tooltip={t('列设置')}
        icon={<IconSetting />}
        onClick={() => setShowColumnSelector(true)}
      />
    </div>
  );
};

export default ChannelsActionRail;
