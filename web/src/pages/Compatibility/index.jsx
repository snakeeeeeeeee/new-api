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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Banner,
  Button,
  Col,
  Form,
  Layout,
  Row,
  Spin,
  TabPane,
  Tabs,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  API,
  compareObjects,
  parseHttpStatusCodeRules,
  showError,
  showSuccess,
  showWarning,
  toBoolean,
  verifyJSON,
} from '../../helpers';
import HttpStatusCodeRulesInput from '../../components/settings/HttpStatusCodeRulesInput';

const { Text } = Typography;

const DEFAULT_OPTIONS = {
  'relay_error_setting.log_upstream_error_detail_enabled': true,
  'relay_error_setting.passthrough_enabled': false,
  'relay_error_setting.passthrough_status_codes': '400,422',
  'relay_error_setting.mask_sensitive': true,
  'claude.apply_compat_in_passthrough_enabled': false,
  'claude.auto_fix_image_media_type_enabled': true,
  'claude.preserve_zero_max_tokens_enabled': true,
  'claude.default_max_tokens': '{"default":8192}',
  'claude.drop_default_sampling_for_opus_enabled': true,
  'claude.validate_output_effort_enabled': true,
  'claude.normalize_simple_message_content_enabled': true,
  'claude.promote_leading_system_role_enabled': true,
  'claude.merge_adjacent_same_role_enabled': true,
  'claude.reorder_tool_result_blocks_enabled': false,
  'claude.openai_tool_call_compat_enabled': true,
  'claude.request_schema_validation_mode': 'reject',
  'claude.tool_protocol_validation_mode': 'reject',
  'claude.tool_schema_validation_mode': 'log',
  'claude.tool_choice_validation_mode': 'log',
  'claude.thinking_validation_mode': 'log',
  'claude.image_limits_validation_mode': 'log',
  'claude.prompt_cache_validation_mode': 'log',
  'claude.stop_sequences_validation_mode': 'reject',
  'claude.service_tier_validation_mode': 'reject',
  'claude.metadata_user_id_validation_mode': 'log',
  'claude.assistant_prefill_validation_mode': 'log',
  'claude.request_size_limit_bytes': '33554432',
  'claude.response_integrity_fallback_enabled': false,
  'claude.response_integrity_first_block_timeout_seconds': '30',
  'global.chat_completions_to_responses_policy': '{}',
  'global.openai_reserved_function_name_compat_enabled': true,
  'global.openai_reserved_function_names': 'python',
  'global.openai_tool_schema_null_required_compat_enabled': false,
};

const CLAUDE_DEFAULT_MAX_TOKENS_EXAMPLE = {
  default: 8192,
  'claude-3-haiku-20240307': 4096,
  'claude-3-opus-20240229': 4096,
  'claude-3-7-sonnet-20250219-thinking': 8192,
};

function normalizeOptionValue(key, value) {
  if (typeof DEFAULT_OPTIONS[key] === 'boolean') {
    return toBoolean(value);
  }
  if (
    key === 'claude.default_max_tokens' ||
    key === 'global.chat_completions_to_responses_policy'
  ) {
    if (!value) return DEFAULT_OPTIONS[key];
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  return value ?? DEFAULT_OPTIONS[key];
}

function optionValueForSubmit(key, value, parsedStatusCodes) {
  if (typeof DEFAULT_OPTIONS[key] === 'boolean') {
    return String(value);
  }
  if (key === 'relay_error_setting.passthrough_status_codes') {
    return parsedStatusCodes.normalized;
  }
  if (
    key === 'claude.default_max_tokens' ||
    key === 'global.chat_completions_to_responses_policy'
  ) {
    return String(value || '').trim();
  }
  if (key === 'global.openai_reserved_function_names') {
    return normalizeOpenAIReservedFunctionNames(value).normalized;
  }
  return value;
}

function normalizeOpenAIReservedFunctionNames(value) {
  const rawValue = String(value || '');
  const names = rawValue
    .split(/[,\r\n]/)
    .map((name) => name.trim())
    .filter(Boolean);
  const uniqueNames = [...new Set(names)];
  const ok =
    new TextEncoder().encode(rawValue).length <= 8192 &&
    uniqueNames.length <= 128 &&
    uniqueNames.every((name) => /^[A-Za-z0-9_-]{1,64}$/.test(name));
  return { ok, normalized: uniqueNames.join('\n') };
}

const VALIDATION_MODE_OPTIONS = [
  { label: '关闭', value: 'off' },
  { label: '仅记录', value: 'log' },
  { label: '拒绝请求', value: 'reject' },
];

function SectionHeader({ title, description }) {
  return (
    <div className='mb-4'>
      <Typography.Title heading={5} style={{ margin: 0 }}>
        {title}
      </Typography.Title>
      {description ? (
        <Text type='secondary' size='small'>
          {description}
        </Text>
      ) : null}
    </div>
  );
}

function SwitchGrid({ fields, inputs, setInputs, t }) {
  return (
    <Row gutter={[16, 12]}>
      {fields.map((field) => (
        <Col xs={24} sm={12} md={8} lg={8} xl={6} key={field.key}>
          <Form.Switch
            field={field.key}
            label={t(field.label)}
            extraText={field.extra ? t(field.extra) : undefined}
            onChange={(value) =>
              setInputs((current) => ({
                ...current,
                [field.key]: value,
              }))
            }
          />
        </Col>
      ))}
    </Row>
  );
}

function ModeGrid({ fields, inputs, setInputs, t }) {
  return (
    <Row gutter={[16, 12]}>
      {fields.map((field) => (
        <Col xs={24} sm={12} md={8} lg={8} xl={6} key={field.key}>
          <Form.Select
            field={field.key}
            label={t(field.label)}
            extraText={field.extra ? t(field.extra) : undefined}
            optionList={VALIDATION_MODE_OPTIONS.map((item) => ({
              label: t(item.label),
              value: item.value,
            }))}
            onChange={(value) =>
              setInputs((current) => ({
                ...current,
                [field.key]: value,
              }))
            }
          />
        </Col>
      ))}
    </Row>
  );
}

export default function CompatibilityPage() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [inputs, setInputs] = useState(DEFAULT_OPTIONS);
  const [inputsRow, setInputsRow] = useState(DEFAULT_OPTIONS);
  const formRef = useRef();

  const parsedPassthroughStatusCodes = parseHttpStatusCodeRules(
    inputs['relay_error_setting.passthrough_status_codes'] || '',
  );

  const generalSwitches = useMemo(
    () => [
      {
        key: 'relay_error_setting.log_upstream_error_detail_enabled',
        label: '记录上游错误详情',
        extra:
          '记录 status、request_id、type、message、code、param，不记录完整请求内容。',
      },
      {
        key: 'relay_error_setting.passthrough_enabled',
        label: '透出上游 400/422 具体原因',
        extra: '状态码范围由下方配置控制。',
      },
      {
        key: 'relay_error_setting.mask_sensitive',
        label: '错误信息脱敏',
        extra: '透出前继续隐藏 URL、IP、密钥等敏感片段。',
      },
      {
        key: 'claude.apply_compat_in_passthrough_enabled',
        label: '透传请求也应用兼容修复',
        extra: '开启后会规范化透传 Claude JSON body。',
      },
    ],
    [],
  );

  const claudeSwitches = useMemo(
    () => [
      {
        key: 'claude.auto_fix_image_media_type_enabled',
        label: '图片 MIME 自动修正',
        extra: '仅修正 base64 image source 的 media_type。',
      },
      {
        key: 'claude.preserve_zero_max_tokens_enabled',
        label: '保留 max_tokens=0 缓存预热语义',
        extra: '显式 0 不补默认值，冲突组合本地返回 400。',
      },
      {
        key: 'claude.drop_default_sampling_for_opus_enabled',
        label: 'Claude Thinking 采样参数清理',
        extra:
          '清理 Opus 4.7/4.8、Fable 5、Sonnet 5 以及已启用 enabled/adaptive thinking 请求的 temperature、top_p、top_k；原生透传时也独立生效，避免上游 400。',
      },
      {
        key: 'claude.validate_output_effort_enabled',
        label: 'output_config.effort 等级校验',
        extra: '校验 low/medium/high/xhigh/max 与模型支持范围。',
      },
      {
        key: 'claude.normalize_simple_message_content_enabled',
        label: '简单 content 自动兼容',
        extra:
          '将 messages[].content 的 null、缺失、空数组、数字、布尔值转换为 Claude 可接受的文本内容；对象类型仍返回 400。',
      },
      {
        key: 'claude.promote_leading_system_role_enabled',
        label: '开头 system/developer 提升',
        extra: '只提升开头连续段，中间 system/developer 不移动。',
      },
      {
        key: 'claude.merge_adjacent_same_role_enabled',
        label: '相邻同 role 合并',
        extra: '只合并 user/user、assistant/assistant。',
      },
      {
        key: 'claude.reorder_tool_result_blocks_enabled',
        label: 'tool_result 简单顺序修正',
        extra: '默认关闭，仅把同一 user 消息内 tool_result 移到 text 前。',
      },
      {
        key: 'claude.openai_tool_call_compat_enabled',
        label: 'OpenAI 工具调用转 Claude 兼容',
        extra:
          '开启后会修复 OpenAI 历史消息中的 tool_calls/tool_call_id 到 Claude tool_use/tool_result 的协议结构，兼容 Codex Desktop 等客户端的历史工具调用格式。',
      },
      {
        key: 'claude.response_integrity_fallback_enabled',
        label: 'Claude 响应完整性保护',
        extra:
          '开启后校验 content block 序列，并在首内容块前异常时交给现有渠道 fallback。默认关闭。',
      },
    ],
    [],
  );

  const claudeValidationModes = useMemo(
    () => [
      {
        key: 'claude.request_schema_validation_mode',
        label: '基础请求结构校验',
        extra: '默认拒绝确定无法构造 Claude 请求的硬错误。',
      },
      {
        key: 'claude.tool_protocol_validation_mode',
        label: '工具调用协议校验',
        extra: '默认拒绝 tool_use/tool_result 明确无法配对的请求。',
      },
      {
        key: 'claude.tool_schema_validation_mode',
        label: '工具 Schema 校验',
        extra: '默认仅记录，避免不同渠道 Schema 兼容差异造成误伤。',
      },
      {
        key: 'claude.tool_choice_validation_mode',
        label: 'tool_choice 校验',
        extra: '默认仅记录 forced tool、name 引用等潜在问题。',
      },
      {
        key: 'claude.thinking_validation_mode',
        label: 'Thinking 参数校验',
        extra: '默认仅记录 budget、prefill、采样参数等冲突。',
      },
      {
        key: 'claude.image_limits_validation_mode',
        label: '图片数量/大小/尺寸限制',
        extra: '默认仅记录，不压缩、不转码、不下载 URL 图片。',
      },
      {
        key: 'claude.prompt_cache_validation_mode',
        label: 'Prompt Cache 校验',
        extra: '默认仅记录 TTL、breakpoint 和 slot 冲突。',
      },
      {
        key: 'claude.stop_sequences_validation_mode',
        label: 'stop_sequences 校验',
        extra: '默认拒绝非字符串数组或空字符串。',
      },
      {
        key: 'claude.service_tier_validation_mode',
        label: 'service_tier 校验',
        extra: '默认拒绝非 auto/standard_only。',
      },
      {
        key: 'claude.metadata_user_id_validation_mode',
        label: 'metadata.user_id 校验',
        extra: '默认仅记录明显 PII 风险。',
      },
      {
        key: 'claude.assistant_prefill_validation_mode',
        label: 'Assistant 预填校验',
        extra:
          '默认仅记录已知不支持 assistant message prefill 的模型，例如 Claude Haiku 4.5。',
      },
    ],
    [],
  );

  async function loadOptions() {
    setLoading(true);
    try {
      const res = await API.get('/api/option/');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const nextInputs = { ...DEFAULT_OPTIONS };
      data.forEach((item) => {
        if (Object.prototype.hasOwnProperty.call(DEFAULT_OPTIONS, item.key)) {
          nextInputs[item.key] = normalizeOptionValue(item.key, item.value);
        }
      });
      setInputs(nextInputs);
      setInputsRow(structuredClone(nextInputs));
      formRef.current?.setValues(nextInputs);
    } catch (error) {
      showError(t('刷新失败'));
    } finally {
      setLoading(false);
    }
  }

  async function saveOptions() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) {
      showWarning(t('你似乎并没有修改什么'));
      return;
    }
    if (!parsedPassthroughStatusCodes.ok) {
      showError(t('错误透传状态码格式不正确'));
      return;
    }
    if (!verifyJSON(inputs['claude.default_max_tokens'])) {
      showError(t('缺省 MaxTokens 不是合法的 JSON 字符串'));
      return;
    }
    if (!verifyJSON(inputs['global.chat_completions_to_responses_policy'])) {
      showError(t('ChatCompletions→Responses 兼容配置不是合法的 JSON 字符串'));
      return;
    }
    if (
      !normalizeOpenAIReservedFunctionNames(
        inputs['global.openai_reserved_function_names'],
      ).ok
    ) {
      showError(t('OpenAI 保留函数名配置格式不正确'));
      return;
    }
    if (Number(inputs['claude.request_size_limit_bytes']) <= 0) {
      showError(t('Claude 请求体限制必须大于 0'));
      return;
    }
    const requestQueue = updateArray.map((item) =>
      API.put('/api/option/', {
        key: item.key,
        value: optionValueForSubmit(
          item.key,
          inputs[item.key],
          parsedPassthroughStatusCodes,
        ),
      }),
    );

    setSaving(true);
    try {
      const res = await Promise.all(requestQueue);
      if (res.includes(undefined)) {
        showError(t('部分保存失败，请重试'));
        return;
      }
      const failedResponse = res.find((item) => item?.data?.success === false);
      if (failedResponse) {
        showError(failedResponse.data.message || t('部分保存失败，请重试'));
        return;
      }
      showSuccess(t('保存成功'));
      await loadOptions();
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setSaving(false);
    }
  }

  useEffect(() => {
    loadOptions();
  }, []);

  return (
    <div className='mt-[60px] px-2'>
      <Layout>
        <Layout.Content>
          <Spin spinning={loading || saving}>
            <div className='mb-3 flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between'>
              <div>
                <Typography.Title heading={4} style={{ margin: 0 }}>
                  {t('兼容管理')}
                </Typography.Title>
                <Text type='secondary'>
                  {t('统一管理协议、模型和渠道兼容策略，原有设置入口仍保留。')}
                </Text>
              </div>
              <Button type='primary' onClick={saveOptions} loading={saving}>
                {t('保存变更')}
              </Button>
            </div>
            <Form
              values={inputs}
              getFormApi={(formAPI) => (formRef.current = formAPI)}
            >
              <Tabs type='card' collapsible>
                <TabPane itemKey='general' tab={t('通用兼容')}>
                  <SectionHeader
                    title={t('通用兼容')}
                    description={t(
                      '控制上游错误详情、错误透出和透传请求的兼容处理。',
                    )}
                  />
                  <SwitchGrid
                    fields={generalSwitches}
                    inputs={inputs}
                    setInputs={setInputs}
                    t={t}
                  />
                  <Row gutter={[16, 12]} className='mt-2'>
                    <Col xs={24} md={14} lg={10}>
                      <HttpStatusCodeRulesInput
                        label={t('错误透传状态码')}
                        placeholder={t('例如：400, 422')}
                        extraText={t(
                          '支持单个状态码或范围。建议仅透传调用方可修复的请求错误。',
                        )}
                        field={'relay_error_setting.passthrough_status_codes'}
                        onChange={(value) =>
                          setInputs((current) => ({
                            ...current,
                            'relay_error_setting.passthrough_status_codes':
                              value,
                          }))
                        }
                        parsed={parsedPassthroughStatusCodes}
                        invalidText={t('错误透传状态码格式不正确')}
                      />
                    </Col>
                  </Row>
                </TabPane>
                <TabPane itemKey='claude' tab={t('Claude 兼容')}>
                  <SectionHeader
                    title={t('Claude 兼容')}
                    description={t(
                      '处理 Claude 高频 400、max_tokens=0、采样参数、effort、role 和 tool_result。',
                    )}
                  />
                  <SwitchGrid
                    fields={claudeSwitches}
                    inputs={inputs}
                    setInputs={setInputs}
                    t={t}
                  />
                  <div className='mt-6'>
                    <SectionHeader
                      title={t('Claude 前置校验模式')}
                      description={t(
                        '保守默认：硬错误直接拒绝，存在误伤风险的规则默认仅记录。',
                      )}
                    />
                    <ModeGrid
                      fields={claudeValidationModes}
                      inputs={inputs}
                      setInputs={setInputs}
                      t={t}
                    />
                  </div>
                  <Row gutter={[16, 12]} className='mt-2'>
                    <Col xs={24} md={12} lg={8}>
                      <Form.InputNumber
                        label={t('首内容块超时')}
                        field={
                          'claude.response_integrity_first_block_timeout_seconds'
                        }
                        min={1}
                        max={300}
                        step={1}
                        disabled={
                          !inputs['claude.response_integrity_fallback_enabled']
                        }
                        extraText={t(
                          '从发起上游请求开始计算，范围 1-300 秒，默认 30 秒；message_start 和 ping 不会重置计时。',
                        )}
                        onChange={(value) =>
                          setInputs((current) => ({
                            ...current,
                            'claude.response_integrity_first_block_timeout_seconds':
                              String(value),
                          }))
                        }
                      />
                    </Col>
                    <Col xs={24} md={12} lg={8}>
                      <Form.InputNumber
                        label={t('Claude 请求体限制')}
                        field={'claude.request_size_limit_bytes'}
                        min={1}
                        step={1048576}
                        extraText={t('单位为字节，默认 33554432，即 32MB。')}
                        onChange={(value) =>
                          setInputs((current) => ({
                            ...current,
                            'claude.request_size_limit_bytes': String(value),
                          }))
                        }
                      />
                    </Col>
                    <Col xs={24} lg={12}>
                      <Form.TextArea
                        label={t('缺省 max_tokens 默认值')}
                        field={'claude.default_max_tokens'}
                        placeholder={JSON.stringify(
                          CLAUDE_DEFAULT_MAX_TOKENS_EXAMPLE,
                          null,
                          2,
                        )}
                        extraText={t(
                          '仅字段缺失时使用；显式 max_tokens=0 不会被覆盖。',
                        )}
                        autosize={{ minRows: 6, maxRows: 12 }}
                        trigger='blur'
                        stopValidateWithError
                        rules={[
                          {
                            validator: (rule, value) => verifyJSON(value),
                            message: t('不是合法的 JSON 字符串'),
                          },
                        ]}
                        onChange={(value) =>
                          setInputs((current) => ({
                            ...current,
                            'claude.default_max_tokens': value,
                          }))
                        }
                      />
                    </Col>
                  </Row>
                </TabPane>
                <TabPane itemKey='openai' tab={t('OpenAI 兼容')}>
                  <SectionHeader
                    title={t('OpenAI 兼容')}
                    description={t(
                      '管理 OpenAI Chat Completions 转换、工具 Schema 及上游保留函数名兼容。',
                    )}
                  />
                  <Row gutter={[16, 12]}>
                    <Col xs={24} md={12} lg={10}>
                      <Form.Switch
                        label={t('空 required Schema 自动清理')}
                        field={
                          'global.openai_tool_schema_null_required_compat_enabled'
                        }
                        extraText={t(
                          '仅递归删除工具参数 Schema 中值为 null 的 required；其他非法值保持不变。默认关闭。',
                        )}
                        onChange={(value) =>
                          setInputs((current) => ({
                            ...current,
                            'global.openai_tool_schema_null_required_compat_enabled':
                              value,
                          }))
                        }
                      />
                    </Col>
                  </Row>
                  <Row gutter={[16, 12]} className='mt-4'>
                    <Col xs={24} md={12} lg={10}>
                      <Form.Switch
                        label={t('保留函数名自动兼容')}
                        field={
                          'global.openai_reserved_function_name_compat_enabled'
                        }
                        extraText={t(
                          '命中后转发为 run_<name>，响应时自动还原原名。',
                        )}
                        onChange={(value) =>
                          setInputs((current) => ({
                            ...current,
                            'global.openai_reserved_function_name_compat_enabled':
                              value,
                          }))
                        }
                      />
                    </Col>
                    <Col xs={24} lg={14}>
                      <Form.TextArea
                        label={t('OpenAI 保留函数名')}
                        field={'global.openai_reserved_function_names'}
                        placeholder={'python\nxxx'}
                        disabled={
                          !inputs[
                            'global.openai_reserved_function_name_compat_enabled'
                          ]
                        }
                        extraText={t(
                          '使用英文逗号或换行分隔；名称仅支持字母、数字、下划线和连字符，每项最多 64 个字符。',
                        )}
                        autosize={{ minRows: 3, maxRows: 8 }}
                        trigger='blur'
                        stopValidateWithError
                        rules={[
                          {
                            validator: (rule, value) =>
                              normalizeOpenAIReservedFunctionNames(value).ok,
                            message: t('OpenAI 保留函数名配置格式不正确'),
                          },
                        ]}
                        onChange={(value) =>
                          setInputs((current) => ({
                            ...current,
                            'global.openai_reserved_function_names': value,
                          }))
                        }
                      />
                    </Col>
                  </Row>
                  <Row gutter={[16, 12]} className='mt-6'>
                    <Col xs={24} lg={14}>
                      <Form.TextArea
                        label={t('ChatCompletions→Responses 兼容配置')}
                        field={'global.chat_completions_to_responses_policy'}
                        placeholder={JSON.stringify({})}
                        extraText={t(
                          '复用现有全局配置，仍可在模型相关设置中管理。',
                        )}
                        autosize={{ minRows: 8, maxRows: 16 }}
                        trigger='blur'
                        stopValidateWithError
                        rules={[
                          {
                            validator: (rule, value) => verifyJSON(value),
                            message: t('不是合法的 JSON 字符串'),
                          },
                        ]}
                        onChange={(value) =>
                          setInputs((current) => ({
                            ...current,
                            'global.chat_completions_to_responses_policy':
                              value,
                          }))
                        }
                      />
                    </Col>
                  </Row>
                </TabPane>
                <TabPane itemKey='channel' tab={t('渠道兼容')}>
                  <SectionHeader
                    title={t('渠道兼容')}
                    description={t(
                      '渠道级差异继续在渠道配置内保存，兼容管理只提供统一说明和入口预留。',
                    )}
                  />
                  <Banner
                    type='info'
                    title={t('Claude 工具 Schema 兼容修复')}
                    description={t(
                      '该开关属于渠道配置 claude_tool_schema_compat_enabled，当前仍在渠道编辑页管理；兼容管理 v1 不迁移原入口。',
                    )}
                  />
                </TabPane>
              </Tabs>
            </Form>
          </Spin>
        </Layout.Content>
      </Layout>
    </div>
  );
}
