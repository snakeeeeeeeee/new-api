import React, { useContext, useEffect, useMemo, useState } from 'react';
import { Button, Spin, Tag, Typography } from '@douyinfe/semi-ui';
import {
  Check,
  CircleAlert,
  Image,
  KeyRound,
  PanelsTopLeft,
  UserRound,
  Video,
} from 'lucide-react';
import { StatusContext } from '../../context/Status';
import { clearAuthReturnTo } from '../../helpers/authReturn';

function requestFromSearch() {
  const params = new URLSearchParams(window.location.search);
  return {
    client_id: params.get('client_id') || '',
    redirect_uri: params.get('redirect_uri') || '',
    state: params.get('state') || '',
    code_challenge: params.get('code_challenge') || '',
    code_challenge_method: params.get('code_challenge_method') || '',
  };
}

function userAuthHeaders() {
  try {
    const user = JSON.parse(localStorage.getItem('user') || 'null');
    return Number.isInteger(user?.id)
      ? { 'New-Api-User': String(user.id) }
      : {};
  } catch {
    return {};
  }
}

function ScopeRow({ icon, title, description }) {
  return (
    <div className='flex gap-3 py-3'>
      <div className='mt-0.5 text-semi-color-text-1'>{icon}</div>
      <div className='min-w-0 flex-1'>
        <div className='text-sm font-medium text-semi-color-text-0'>
          {title}
        </div>
        <div className='mt-0.5 text-xs leading-5 text-semi-color-text-2'>
          {description}
        </div>
      </div>
      <Check size={16} className='mt-1 text-green-600' />
    </div>
  );
}

export default function CanvasAuthorizePage() {
  const [statusState] = useContext(StatusContext);
  const [loading, setLoading] = useState(true);
  const [approving, setApproving] = useState(false);
  const [context, setContext] = useState(null);
  const [error, setError] = useState(null);
  const request = useMemo(requestFromSearch, []);
  const systemName = statusState?.status?.system_name || 'SuperToken';
  const logo = statusState?.status?.logo;

  useEffect(() => {
    let active = true;
    const load = async () => {
      let redirecting = false;
      try {
        const response = await fetch(
          `/api/canvas/authorization/context${window.location.search}`,
          {
            credentials: 'include',
            cache: 'no-store',
            headers: userAuthHeaders(),
          },
        );
        if (response.status === 401) {
          redirecting = true;
          localStorage.removeItem('user');
          const returnTo = `${window.location.pathname}${window.location.search}`;
          window.location.replace(
            `/login?return_to=${encodeURIComponent(returnTo)}`,
          );
          return;
        }
        clearAuthReturnTo();
        const body = await response.json();
        if (!active) return;
        if (!body.success) setError(body);
        else setContext(body.data);
      } catch {
        if (active) setError({ message: '无法连接授权服务，请稍后重试' });
      } finally {
        if (active && !redirecting) setLoading(false);
      }
    };
    load();
    return () => {
      active = false;
    };
  }, []);

  const finish = (params) => {
    if (!context) return;
    const target = new URL(request.redirect_uri);
    Object.entries(params).forEach(([key, value]) =>
      target.searchParams.set(key, value),
    );
    window.location.replace(target.toString());
  };

  const approve = async () => {
    setApproving(true);
    setError(null);
    try {
      const response = await fetch('/api/canvas/authorization/code', {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          ...userAuthHeaders(),
        },
        body: JSON.stringify(request),
      });
      const body = await response.json();
      if (!body.success) setError(body);
      else finish({ code: body.data.code, state: body.data.state });
    } catch {
      setError({ message: '授权请求失败，请稍后重试' });
    } finally {
      setApproving(false);
    }
  };

  const cancel = () => {
    if (!context) {
      window.close();
      return;
    }
    finish({
      error: 'access_denied',
      error_description: '用户取消了授权',
      state: request.state,
    });
  };

  return (
    <main
      className='flex min-h-screen items-center justify-center px-4 py-8'
      style={{ background: 'var(--semi-color-bg-1)' }}
    >
      <div className='w-full max-w-[520px]'>
        <div className='mb-5 flex items-center justify-center gap-3'>
          <div
            className='flex size-12 items-center justify-center rounded-xl border'
            style={{
              borderColor: 'var(--semi-color-border)',
              background: 'var(--semi-color-bg-0)',
            }}
          >
            <PanelsTopLeft size={24} />
          </div>
          <div>
            <Typography.Title heading={4} style={{ margin: 0 }}>
              Infinite Canvas
            </Typography.Title>
            <Typography.Text type='secondary' size='small'>
              请求连接 {systemName}
            </Typography.Text>
          </div>
        </div>
        <section
          className='overflow-hidden rounded-lg border'
          style={{
            borderColor: 'var(--semi-color-border)',
            background: 'var(--semi-color-bg-0)',
            boxShadow: '0 12px 30px rgba(0, 0, 0, 0.06)',
          }}
        >
          {loading ? (
            <div className='flex min-h-[420px] items-center justify-center'>
              <Spin size='large' />
            </div>
          ) : error ? (
            <div className='px-6 py-10 text-center'>
              <CircleAlert size={34} className='mx-auto text-red-500' />
              <Typography.Title heading={5} className='mt-4'>
                无法完成授权
              </Typography.Title>
              <Typography.Text type='secondary'>
                {error.message}
              </Typography.Text>
              {error.missing_groups?.length ? (
                <div className='mt-5 flex flex-wrap justify-center gap-2'>
                  {error.missing_groups.map((group) => (
                    <Tag key={group} color='red'>
                      缺少分组：{group}
                    </Tag>
                  ))}
                </div>
              ) : null}
              <Button className='mt-7' onClick={cancel}>
                关闭
              </Button>
            </div>
          ) : (
            <>
              <div className='border-b border-semi-color-border px-6 py-5'>
                <div className='flex items-center gap-3'>
                  {logo ? (
                    <img
                      src={logo}
                      alt=''
                      className='size-9 rounded-lg object-cover'
                    />
                  ) : (
                    <KeyRound size={22} />
                  )}
                  <div className='min-w-0'>
                    <div className='text-sm font-medium text-semi-color-text-0'>
                      授权 Infinite Canvas 使用你的账号
                    </div>
                    <div className='mt-0.5 flex items-center gap-1.5 text-xs text-semi-color-text-2'>
                      <UserRound size={13} />
                      <span className='truncate'>
                        {context.display_name || context.username}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
              <div className='px-6 py-3'>
                <ScopeRow
                  icon={<Image size={18} />}
                  title='创建图片生成任务'
                  description={`${context.image_group_name} · ${context.image_models.length} 个可用模型`}
                />
                <ScopeRow
                  icon={<Video size={18} />}
                  title='创建视频生成任务'
                  description={`${context.video_group_name} · ${context.video_models.length} 个可用模型`}
                />
                <ScopeRow
                  icon={<KeyRound size={18} />}
                  title='读取生成资源与任务状态'
                  description='使用账号当前 Resource Key；没有时自动创建长期凭证'
                />
              </div>
              <div className='border-t border-semi-color-border px-6 py-5'>
                <div className='grid grid-cols-2 gap-3'>
                  <Button block size='large' onClick={cancel}>
                    取消
                  </Button>
                  <Button
                    block
                    size='large'
                    theme='solid'
                    type='primary'
                    loading={approving}
                    onClick={approve}
                  >
                    授权 Infinite Canvas
                  </Button>
                </div>
                <div className='mt-4 text-center text-xs leading-5 text-semi-color-text-2'>
                  授权会创建两枚长期模型令牌。你可以在令牌管理中查看它们。
                </div>
              </div>
            </>
          )}
        </section>
      </div>
    </main>
  );
}
