// #48 相位校验单测：轮次间隔必须能命中峰值窗口，否则峰值静默失效。
// 运行：node --test hack/night-run/day-watch.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { phaseHitsPeak } from './day-watch.mjs';

test('interval=900/cycle=30/peak=10：相位错位，峰值永不生效（#48 实测场景）', () => {
  assert.equal(phaseHitsPeak(15, 30, 10), false);
});

test('interval=600/cycle=30/peak=10：10 分钟轮次命中峰值起点', () => {
  assert.equal(phaseHitsPeak(10, 30, 10), true);
});

test('interval=900/cycle=60/peak=15：15 分钟轮次命中峰值起点（原标准剧本）', () => {
  assert.equal(phaseHitsPeak(15, 60, 15), true);
});

test('interval=900/cycle=60/peak=45：轮次 45 落在峰值窗口内', () => {
  assert.equal(phaseHitsPeak(15, 60, 45), true);
});

test('interval=300/cycle=30/peak=10：5 分钟轮次必命中', () => {
  assert.equal(phaseHitsPeak(5, 30, 10), true);
});

test('非法参数返回 false', () => {
  assert.equal(phaseHitsPeak(0, 30, 10), false);
  assert.equal(phaseHitsPeak(10, 30, 0), false);
  assert.equal(phaseHitsPeak(10, 30, 30), false);
});