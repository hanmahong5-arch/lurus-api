import { describe, it, expect } from 'vitest';

describe('vitest harness smoke', () => {
  it('runs', () => {
    expect(1 + 1).toBe(2);
  });

  it('has jsdom env', () => {
    expect(typeof window).toBe('object');
    expect(typeof document).toBe('object');
  });
});
