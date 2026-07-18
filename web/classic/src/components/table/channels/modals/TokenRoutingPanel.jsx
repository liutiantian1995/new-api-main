import React from 'react';
import { useTranslation } from 'react-i18next';
import {
  InputNumber,
  Button,
  Banner,
  Space,
  Divider,
  Row,
  Col,
  Typography,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete } from '@douyinfe/semi-icons';

const { Text } = Typography;

// TokenRoutingPanel — 让管理员为单个渠道配置 token 感知路由策略。
//
// 字段：
//   max_tokens  : int   — 该渠道能稳定承载的最大 estTokens；超过则软过滤跳过该渠道。0=不限。
//   token_tiers : [{max_tokens, priority_boost}]
//                          — 当请求 estTokens ≤ tier.max_tokens 时，priority_boost 累加到 effective_priority。
//
// 该面板默认收起，未配置时不影响现有路由（行为同 main 分支）。
//
// 受控模式：value/onChange 由父表单注入。父组件需把 max_tokens、token_tiers 作为顶层 channel 字段提交。
export default function TokenRoutingPanel({ maxTokens, tokenTiers, onChange }) {
  const { t } = useTranslation();

  const tiers = Array.isArray(tokenTiers) ? tokenTiers : [];

  const emit = (next) => onChange?.(next);

  const updateMaxTokens = (val) => {
    const v = Number.isFinite(val) ? Math.max(0, Math.floor(val)) : 0;
    emit({ maxTokens: v, tokenTiers: tiers });
  };

  const addTier = () => {
    emit({
      maxTokens,
      tokenTiers: [...tiers, { max_tokens: 50000, priority_boost: 0 }],
    });
  };

  const updateTier = (idx, field, value) => {
    const next = tiers.map((tier, i) =>
      i === idx ? { ...tier, [field]: value } : tier,
    );
    emit({ maxTokens, tokenTiers: next });
  };

  const removeTier = (idx) => {
    emit({
      maxTokens,
      tokenTiers: tiers.filter((_, i) => i !== idx),
    });
  };

  return (
    <div className='py-3 border-b border-gray-100'>
      <Text className='text-sm font-medium text-gray-500 mb-3 block'>
        {t('Token 路由策略')}
      </Text>

      <Banner
        type='info'
        description={t(
          '按请求估算 token 数动态调整路由：超过 max_tokens 的请求跳过此渠道；小于分档阈值的请求提升 effective priority。不配置时维持原有路由行为。',
        )}
        fullMode={false}
        closeIcon={null}
        className='mb-3'
      />

      <div className='mb-2'>
        <Text className='text-sm text-gray-600 block mb-1'>
          {t('最大 Token (max_tokens)')}
        </Text>
        <InputNumber
          value={maxTokens}
          placeholder={t('0 表示不限')}
          min={0}
          onNumberChange={updateMaxTokens}
          style={{ width: '100%' }}
        />
        <Text type='tertiary' size='small'>
          {t('estTokens 超过 max_tokens 的请求将跳过此渠道')}
        </Text>
      </div>

      <Divider margin={12} align='left'>
        {t('Token 分档 (token_tiers)')}
      </Divider>

      {tiers.length === 0 && (
        <Banner
          type='neutral'
          description={t('未配置分档。点击「添加分档」为小请求提供 priority_boost。')}
          fullMode={false}
          closeIcon={null}
          className='mb-3'
        />
      )}

      {tiers.map((tier, idx) => (
        <Row key={idx} gutter={8} className='mb-2' align='center'>
          <Col span={10}>
            <InputNumber
              prefix={t('≤ tokens')}
              min={1}
              value={tier.max_tokens}
              onChange={(v) =>
                updateTier(idx, 'max_tokens', Number.isFinite(v) ? Math.max(1, Math.floor(v)) : 1)
              }
              style={{ width: '100%' }}
            />
          </Col>
          <Col span={10}>
            <InputNumber
              prefix={t('priority Δ')}
              min={-100}
              max={100}
              value={tier.priority_boost}
              onChange={(v) =>
                updateTier(idx, 'priority_boost', Number.isFinite(v) ? Math.floor(v) : 0)
              }
              style={{ width: '100%' }}
            />
          </Col>
          <Col span={4}>
            <Button
              theme='borderless'
              type='danger'
              icon={<IconDelete />}
              onClick={() => removeTier(idx)}
            />
          </Col>
        </Row>
      ))}

      <Space>
        <Button
          theme='light'
          icon={<IconPlus />}
          onClick={addTier}
          disabled={tiers.length >= 10}
        >
          {t('添加分档')}
        </Button>
        {tiers.length >= 10 && (
          <span className='text-xs text-gray-400'>
            {t('最多 10 个分档')}
          </span>
        )}
      </Space>
    </div>
  );
}
