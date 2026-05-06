import { describe, expect, it, beforeEach } from 'vitest';
import {
  registerControlTransport,
  unregisterControlTransport,
  getControlTransport,
  __clearControlTransportRegistryForTests,
} from '@/lib/mirror/transport-registry';

describe('transport-registry', () => {
  beforeEach(() => {
    __clearControlTransportRegistryForTests();
  });

  it('register / lookup / unregister round-trip', () => {
    const t = { sendControl: () => {} };
    registerControlTransport(7, t);
    expect(getControlTransport(7)).toBe(t);
    unregisterControlTransport(7, t);
    expect(getControlTransport(7)).toBeUndefined();
  });

  it('unregister only removes if identity matches (avoid race on reconnect)', () => {
    const t1 = { sendControl: () => {} };
    const t2 = { sendControl: () => {} };
    registerControlTransport(7, t1);
    registerControlTransport(7, t2);
    unregisterControlTransport(7, t1);
    expect(getControlTransport(7)).toBe(t2);
  });
});
