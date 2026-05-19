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
import React, { useCallback, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';
import { API, showError } from '../../../helpers';

/* Wave 2: Chat wired to non-stream POST /api/v2/:slug/chat/send.
   In-memory conversation only — no chat_session table yet, so the
   sidebar lists the *current* session (started + turn count), not a
   server-side history. SSE streaming, branching/retry, and
   multi-conversation rehydrate are deferred to v3. */

const DEFAULT_MODEL = 'gpt-4o';

const formatPreview = (text) => {
  if (!text) return '';
  return text.length > 36 ? text.slice(0, 36) + '…' : text;
};

const HFChat = () => {
  const params = useParams();
  const tenantSlug = params.tenant_slug || params.tenantSlug || 'default';

  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [model] = useState(DEFAULT_MODEL);
  const [sessionStartedAt] = useState(() => Date.now());

  const sessionTitle = useMemo(() => {
    const firstUser = messages.find((m) => m.role === 'user');
    return firstUser ? formatPreview(firstUser.content) : 'new chat';
  }, [messages]);

  const turnCount = useMemo(
    () => messages.filter((m) => m.role === 'assistant').length,
    [messages],
  );

  const sessionStartedAgo = useMemo(() => {
    const seconds = Math.floor((Date.now() - sessionStartedAt) / 1000);
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
    return `${Math.floor(seconds / 3600)}h`;
  }, [sessionStartedAt, messages]);

  const send = useCallback(async () => {
    const text = input.trim();
    if (!text || sending) return;

    const nextMessages = [...messages, { role: 'user', content: text }];
    setMessages(nextMessages);
    setInput('');
    setSending(true);

    try {
      const res = await API.post(`/api/v2/${tenantSlug}/chat/send`, {
        model,
        messages: nextMessages.map(({ role, content }) => ({ role, content })),
      });
      const data = res?.data?.data;
      if (!data?.message) {
        throw new Error(res?.data?.message || 'invalid chat response');
      }
      setMessages((prev) => [
        ...prev,
        {
          role: 'assistant',
          content: data.message.content,
          meta: {
            latency_ms: data.latency_ms,
            prompt_tokens: data.usage?.prompt_tokens,
            completion_tokens: data.usage?.completion_tokens,
          },
        },
      ]);
    } catch (err) {
      showError(err?.response?.data?.message || err?.message || '发送失败');
      // Roll back the optimistic user message on failure so the next
      // retry doesn't double-send.
      setMessages(messages);
    } finally {
      setSending(false);
    }
  }, [input, messages, model, sending, tenantSlug]);

  const onKeyDown = useCallback(
    (e) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        send();
      }
    },
    [send],
  );

  return (
    <HFShell
      active='chat'
      crumbs={['workspace', 'chat']}
      actions={
        <>
          <button type='button' className='btn'>
            share ↗
          </button>
          <button type='button' className='btn'>
            ⋯
          </button>
        </>
      }
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '260px 1fr',
          height: '100%',
          minHeight: 0,
        }}
      >
        <div
          style={{
            borderRight: '1px solid var(--hf-rule)',
            background: 'var(--hf-paper)',
            overflow: 'auto',
          }}
        >
          <div style={{ padding: '14px 16px' }}>
            <button
              type='button'
              className='btn primary'
              style={{ width: '100%', justifyContent: 'center' }}
              onClick={() => setMessages([])}
            >
              + new chat
            </button>
          </div>
          <div className='lbl' style={{ padding: '8px 16px' }}>
            current session
          </div>
          <div
            data-testid='session-row'
            style={{
              padding: '10px 16px',
              background: 'var(--hf-elev)',
              borderLeft: '2px solid var(--hf-accent)',
              borderBottom: '1px solid var(--hf-rule)',
            }}
          >
            <div
              className='strong'
              style={{
                fontSize: 12,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {sessionTitle}
            </div>
            <div className='faint mono' style={{ fontSize: 10, marginTop: 2 }}>
              {sessionStartedAgo} ago · {turnCount} turn
              {turnCount === 1 ? '' : 's'}
            </div>
          </div>
          <div style={{ padding: '14px 16px' }}>
            <WIPBanner
              mini
              reason='persistence + streaming deferred to v3'
              todo='no chat_session table yet'
            />
          </div>
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateRows: 'auto 1fr auto',
            minHeight: 0,
          }}
        >
          <div
            style={{
              padding: '12px 22px',
              borderBottom: '1px solid var(--hf-rule)',
              display: 'flex',
              alignItems: 'center',
              gap: 10,
            }}
          >
            <div>
              <div className='display' style={{ fontSize: 17 }}>
                {sessionTitle}
              </div>
              <div className='muted mono' style={{ fontSize: 10 }}>
                {turnCount} turn{turnCount === 1 ? '' : 's'} · started{' '}
                {sessionStartedAgo} ago
              </div>
            </div>
            <span style={{ flex: 1 }} />
            <span className='pill'>
              <span className='dot ok' /> {model}
            </span>
          </div>

          <div
            style={{
              overflow: 'auto',
              padding: '24px 22px',
              maxWidth: 800,
              margin: '0 auto',
              width: '100%',
            }}
          >
            {messages.length === 0 && (
              <div
                className='muted'
                style={{ textAlign: 'center', padding: '60px 0' }}
              >
                ask anything to begin
              </div>
            )}
            {messages.map((m, i) => (
              <div key={i} style={{ marginBottom: 22 }}>
                <div
                  className='lbl'
                  style={{
                    marginBottom: 6,
                    color:
                      m.role === 'user'
                        ? 'var(--hf-ink-3)'
                        : 'var(--hf-accent)',
                  }}
                >
                  {m.role === 'user' ? 'you' : model}
                </div>
                <div
                  style={{
                    fontSize: 14,
                    lineHeight: 1.65,
                    color: 'var(--hf-ink)',
                    whiteSpace: 'pre-wrap',
                  }}
                >
                  {m.content}
                </div>
                {m.meta && (
                  <div
                    style={{
                      display: 'flex',
                      gap: 12,
                      marginTop: 8,
                      fontSize: 10,
                    }}
                  >
                    <span className='faint mono'>
                      {m.meta.latency_ms ?? 0}ms
                    </span>
                    <span className='faint mono'>
                      {(m.meta.prompt_tokens ?? 0) +
                        (m.meta.completion_tokens ?? 0)}
                      t
                    </span>
                    <span style={{ flex: 1 }} />
                    <button
                      type='button'
                      className='btn ghost sm'
                      onClick={() => navigator.clipboard?.writeText(m.content)}
                    >
                      copy
                    </button>
                    <WIPBanner mini reason='retry / branch deferred to v3' />
                  </div>
                )}
              </div>
            ))}
            {sending && (
              <div className='muted' data-testid='sending-indicator'>
                {model} is thinking…
              </div>
            )}
          </div>

          <div
            style={{
              padding: 22,
              borderTop: '1px solid var(--hf-rule)',
              background: 'var(--hf-paper)',
            }}
          >
            <div
              className='panel'
              style={{ padding: 14, maxWidth: 800, margin: '0 auto' }}
            >
              <textarea
                aria-label='message-input'
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={onKeyDown}
                placeholder='ask anything…'
                disabled={sending}
                style={{
                  width: '100%',
                  minHeight: 48,
                  fontSize: 13,
                  border: 'none',
                  outline: 'none',
                  background: 'transparent',
                  resize: 'vertical',
                  color: 'var(--hf-ink)',
                  fontFamily: 'inherit',
                }}
              />
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  marginTop: 8,
                }}
              >
                <span style={{ flex: 1 }} />
                <span className='muted mono' style={{ fontSize: 10 }}>
                  {input.length} / 200,000
                </span>
                <button
                  type='button'
                  className='btn primary'
                  onClick={send}
                  disabled={sending || !input.trim()}
                >
                  ▶ send{' '}
                  <span className='kbd' style={{ marginLeft: 4 }}>
                    ↵
                  </span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </HFShell>
  );
};

export default HFChat;
