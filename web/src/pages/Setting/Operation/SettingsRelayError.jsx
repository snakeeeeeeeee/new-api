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

import React, { useEffect, useRef, useState } from 'react';
import { Banner, Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  API,
  compareObjects,
  parseHttpStatusCodeRules,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import HttpStatusCodeRulesInput from '../../../components/settings/HttpStatusCodeRulesInput';

export default function SettingsRelayError(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'relay_error_setting.passthrough_enabled': false,
    'relay_error_setting.passthrough_status_codes': '400,422',
    'relay_error_setting.passthrough_block_keywords': '',
    'relay_error_setting.mask_sensitive': true,
  });
  const [inputsRow, setInputsRow] = useState(inputs);
  const refForm = useRef();
  const parsedPassthroughStatusCodes = parseHttpStatusCodeRules(
    inputs['relay_error_setting.passthrough_status_codes'] || '',
  );

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    if (!parsedPassthroughStatusCodes.ok) {
      const details =
        parsedPassthroughStatusCodes.invalidTokens &&
        parsedPassthroughStatusCodes.invalidTokens.length > 0
          ? `: ${parsedPassthroughStatusCodes.invalidTokens.join(', ')}`
          : '';
      return showError(`${t('错误透传状态码格式不正确')}${details}`);
    }

    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else {
        const normalizedMap = {
          'relay_error_setting.passthrough_status_codes':
            parsedPassthroughStatusCodes.normalized,
        };
        value = normalizedMap[item.key] ?? inputs[item.key];
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });

    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined))
            return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current.setValues(currentInputs);
  }, [props.options]);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('错误响应设置')}>
            <Banner
              type='warning'
              description={t(
                '启用后，指定状态码的上游错误会返回给调用方，便于定位请求参数问题。建议仅配置 400、422；401、403、429、5xx 可能包含渠道或上游账号信息，不建议透传。',
              )}
              style={{ marginBottom: 16 }}
            />
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'relay_error_setting.passthrough_enabled'}
                  label={t('启用上游请求错误透传')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'relay_error_setting.passthrough_enabled': value,
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'relay_error_setting.mask_sensitive'}
                  label={t('透传错误敏感信息脱敏')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'relay_error_setting.mask_sensitive': value,
                    })
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={16}>
                <HttpStatusCodeRulesInput
                  label={t('错误透传状态码')}
                  placeholder={t('例如：400, 422')}
                  extraText={t(
                    '支持填写单个状态码或范围（含首尾），使用逗号分隔。建议仅透传调用方可修复的请求错误。',
                  )}
                  field={'relay_error_setting.passthrough_status_codes'}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'relay_error_setting.passthrough_status_codes': value,
                    })
                  }
                  parsed={parsedPassthroughStatusCodes}
                  invalidText={t('错误透传状态码格式不正确')}
                />
                <Form.TextArea
                  label={t('错误透传阻断关键词')}
                  placeholder={t('一行一个，不区分大小写')}
                  extraText={t(
                    '状态码允许透传时，如果上游错误内容包含这些关键词，将返回通用错误，不透传原始错误。',
                  )}
                  field={'relay_error_setting.passthrough_block_keywords'}
                  autosize={{ minRows: 4, maxRows: 10 }}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'relay_error_setting.passthrough_block_keywords': value,
                    })
                  }
                />
              </Col>
            </Row>
            <Row>
              <Button size='default' onClick={onSubmit}>
                {t('保存错误响应设置')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
