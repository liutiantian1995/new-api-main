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
  Button,
  SideSheet,
  Space,
  Spin,
  Typography,
  Card,
  Tag,
  Avatar,
  Form,
  Row,
  Col,
  Select,
  Switch,
} from '@douyinfe/semi-ui';
import { IconSave, IconClose, IconUserAdd } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { convertUSDToCurrency } from '../../../../helpers/render';

const { Title } = Typography;

// 默认日期后缀：当天 MMDD（如 0705），基于浏览器本地时区
const getDefaultDateSuffix = () => {
  const now = new Date();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const day = String(now.getDate()).padStart(2, '0');
  return `${month}${day}`;
};

const BatchCreateUserModal = (props) => {
  const { t } = useTranslation();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const [plansLoading, setPlansLoading] = useState(false);
  const [plans, setPlans] = useState([]);
  const isMobile = useIsMobile();

  const getInitValues = () => ({
    prefix: '',
    // 默认当天 MMDD（如 0705）
    date_suffix: getDefaultDateSuffix(),
    count: 10,
    group: '',
    plan_id: undefined,
    activation_strategy: 'immediate',
    create_token: false,
  });

  const planOptions = useMemo(() => {
    return (plans || []).map((p) => ({
      label: `${p?.plan?.title || ''} (${convertUSDToCurrency(
        Number(p?.plan?.price_amount || 0),
        2,
      )})`,
      value: p?.plan?.id,
    }));
  }, [plans]);

  const loadPlans = async () => {
    setPlansLoading(true);
    try {
      const res = await API.get('/api/subscription/admin/plans');
      if (res.data?.success) {
        setPlans(res.data.data || []);
      } else {
        showError(res.data?.message || t('加载套餐失败'));
      }
    } catch (e) {
      showError(t('加载套餐失败'));
    } finally {
      setPlansLoading(false);
    }
  };

  useEffect(() => {
    if (props.visible) {
      loadPlans();
      formApiRef.current?.setValues(getInitValues());
    }
  }, [props.visible]);

  const submit = async (values) => {
    if (!values.prefix) {
      showError(t('请输入用户名前缀'));
      return;
    }
    const count = Number(values.count);
    if (!count || count < 1 || count > 200) {
      showError(t('数量必须在 1 到 200 之间'));
      return;
    }
    setLoading(true);
    try {
      const payload = {
        prefix: values.prefix,
        date_suffix: values.date_suffix || '',
        count,
        group: values.group || '',
        plan_id: values.plan_id || 0,
        activation_strategy: values.activation_strategy || 'immediate',
        create_token: !!values.create_token,
      };
      const res = await API.post('/api/user/batch', payload);
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(
          data?.message ||
            t('批量创建成功，初始密码为 用户名@123'),
        );
        formApiRef.current?.setValues(getInitValues());
        props.refresh();
        props.handleClose();
      } else {
        showError(message);
      }
    } catch (e) {
      showError(t('批量创建失败'));
    } finally {
      setLoading(false);
    }
  };

  const handleCancel = () => {
    props.handleClose();
  };

  return (
    <SideSheet
      placement={'left'}
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('批量')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('批量创建用户')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={props.visible}
      width={isMobile ? '100%' : 600}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconSave />}
              loading={loading}
            >
              {t('批量创建')}
            </Button>
            <Button
              theme='light'
              type='primary'
              onClick={handleCancel}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
      closeIcon={null}
      onCancel={() => handleCancel()}
    >
      <Spin spinning={loading || plansLoading}>
        <Form
          key='batch-create-form'
          initValues={getInitValues()}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={submit}
          onSubmitFail={(errs) => {
            const first = Object.values(errs)[0];
            if (first) showError(Array.isArray(first) ? first[0] : first);
            formApiRef.current?.scrollToError();
          }}
        >
          <div className='p-2'>
            <Card className='!rounded-2xl shadow-sm border-0'>
              <div className='flex items-center mb-2'>
                <Avatar size='small' color='blue' className='mr-2 shadow-md'>
                  <IconUserAdd size={16} />
                </Avatar>
                <div>
                  <Typography.Text className='text-lg font-medium'>
                    {t('批量用户信息')}
                  </Typography.Text>
                  <div className='text-xs text-gray-600'>
                    {t('用户名格式为 前缀+日期+6位随机字符，自动避免重复')}
                  </div>
                </div>
              </div>

              <Row gutter={12}>
                <Col span={24}>
                  <Form.Input
                    field='prefix'
                    label={t('用户名前缀')}
                    placeholder={t('例如 user')}
                    rules={[
                      { required: true, message: t('请输入用户名前缀') },
                      {
                        max: 10,
                        message: t('前缀最多 10 个字符'),
                      },
                    ]}
                    showClear
                  />
                </Col>
                <Col span={24}>
                  <Form.Input
                    field='date_suffix'
                    label={t('日期后缀')}
                    placeholder={t('例如 0601（MMDD），默认当天')}
                    rules={[{ max: 8, message: t('最多 8 个字符') }]}
                    showClear
                  />
                </Col>
                <Col span={24}>
                  <Form.InputNumber
                    field='count'
                    label={t('创建数量')}
                    placeholder={t('一次最多创建 200 个用户')}
                    min={1}
                    max={200}
                    rules={[
                      { required: true, message: t('请输入创建数量') },
                      {
                        validator: (rule, v) =>
                          v >= 1 && v <= 200
                            ? true
                            : t('数量必须在 1 到 200 之间'),
                      },
                    ]}
                    showClear
                  />
                </Col>
                <Col span={24}>
                  <Form.Input
                    field='group'
                    label={t('用户分组')}
                    placeholder={t('留空使用默认分组 default')}
                    showClear
                  />
                </Col>
                <Col span={24}>
                  <Form.Select
                    field='plan_id'
                    label={t('订阅套餐（可选）')}
                    placeholder={t('不绑定订阅套餐')}
                    optionList={planOptions}
                    emptyContent={t('暂无可用套餐')}
                    showClear
                  />
                </Col>
                <Col span={24}>
                  <Form.Select
                    field='activation_strategy'
                    label={t('生效策略')}
                    optionList={[
                      { label: t('立即生效'), value: 'immediate' },
                      { label: t('使用时生效'), value: 'on_use' },
                    ]}
                    helpText={t(
                      '使用时生效：用户首次登录或使用 API 密钥时才开始计时',
                    )}
                  />
                </Col>
                <Col span={24}>
                  <Form.Switch
                    field='create_token'
                    label={t('创建 API 密钥')}
                    helpText={t(
                      '为每个用户创建一个不限额度的默认 API 密钥',
                    )}
                  >
                    {(prop) => (
                      <Switch
                        checked={prop.value}
                        onChange={(v) => prop.onChange(v)}
                      />
                    )}
                  </Form.Switch>
                </Col>
              </Row>
            </Card>
          </div>
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default BatchCreateUserModal;
