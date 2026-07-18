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
import { Button, Card, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import {
  API,
  compareObjects,
  showError,
  showSuccess,
  showWarning,
  toBoolean,
  verifyJSON,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const OPTION_KEYS = [
  'UserRollingRateLimitEnabled',
  'UserRollingRateLimitGroup',
];

export default function SettingsRollingRateLimit() {
  const { t } = useTranslation();

  const [loading, setLoading] = useState(false);
  const [fetching, setFetching] = useState(false);
  const [inputs, setInputs] = useState({
    UserRollingRateLimitEnabled: false,
    UserRollingRateLimitGroup: '',
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  async function getOptions() {
    try {
      setFetching(true);
      const res = await API.get('/api/option/');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const newInputs = {
        UserRollingRateLimitEnabled: false,
        UserRollingRateLimitGroup: '',
      };
      data.forEach((item) => {
        if (!OPTION_KEYS.includes(item.key)) return;
        if (item.key === 'UserRollingRateLimitGroup') {
          try {
            newInputs[item.key] = JSON.stringify(
              JSON.parse(item.value),
              null,
              2,
            );
          } catch {
            newInputs[item.key] = item.value;
          }
        } else if (item.key.endsWith('Enabled')) {
          newInputs[item.key] = toBoolean(item.value);
        } else {
          newInputs[item.key] = item.value;
        }
      });
      setInputs(newInputs);
      setInputsRow(structuredClone(newInputs));
      refForm.current?.setValues(newInputs);
    } catch (error) {
      showError(t('刷新失败'));
    } finally {
      setFetching(false);
    }
  }

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else {
        value = inputs[item.key];
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

        for (let i = 0; i < res.length; i++) {
          if (!res[i].data.success) {
            return showError(res[i].data.message);
          }
        }

        showSuccess(t('保存成功'));
        getOptions();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    getOptions();
  }, []);

  return (
    <>
      <Spin spinning={fetching} size='large'>
        <Card style={{ marginTop: '10px' }}>
          <Spin spinning={loading}>
            <Form
              values={inputs}
              getFormApi={(formAPI) => (refForm.current = formAPI)}
              style={{ marginBottom: 15 }}
            >
              <Form.Section text={t('用户滚动配额')}>
                <Row gutter={16}>
                  <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                    <Form.Switch
                      field={'UserRollingRateLimitEnabled'}
                      label={t('启用用户滚动配额')}
                      size='default'
                      checkedText='｜'
                      uncheckedText='〇'
                      onChange={(value) => {
                        setInputs({
                          ...inputs,
                          UserRollingRateLimitEnabled: value,
                        });
                      }}
                    />
                  </Col>
                </Row>
                <Row>
                  <Col xs={24} sm={16}>
                    <Form.TextArea
                      label={t('用户滚动配额分组配置')}
                      placeholder={t(
                        '{\n  "default": [{"duration": 18000, "limit": 500}]\n}',
                      )}
                      field={'UserRollingRateLimitGroup'}
                      autosize={{ minRows: 5, maxRows: 15 }}
                      trigger='blur'
                      stopValidateWithError
                      rules={[
                        {
                          validator: (rule, value) => verifyJSON(value),
                          message: t('不是合法的 JSON 字符串'),
                        },
                      ]}
                      extraText={
                        <div>
                          <p>
                            {t(
                              '补充分钟级速率限制：控制长期请求配额（5h/1d/1w）。',
                            )}
                          </p>
                          <p>{t('滚动配额 JSON 格式')}</p>
                          <ul>
                            <li>
                              {t(
                                '使用 JSON 对象格式，键为分组名，值为配额规则数组。',
                              )}
                            </li>
                            <li>
                              {t(
                                '示例：{"default": [{"duration": 18000, "limit": 500}]}。',
                              )}
                            </li>
                            <li>
                              {t(
                                'duration 单位为秒，limit 表示该时间窗口内的最大请求次数。',
                              )}
                            </li>
                          </ul>
                        </div>
                      }
                      onChange={(value) => {
                        setInputs({
                          ...inputs,
                          UserRollingRateLimitGroup: value,
                        });
                      }}
                    />
                  </Col>
                </Row>
                <Row>
                  <Button size='default' onClick={onSubmit}>
                    {t('保存用户滚动配额')}
                  </Button>
                </Row>
              </Form.Section>
            </Form>
          </Spin>
        </Card>
      </Spin>
    </>
  );
}
