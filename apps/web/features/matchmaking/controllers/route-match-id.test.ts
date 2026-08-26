import { describe, expect, it } from 'vitest';
import { normalizeRouteMatchId } from '../../../pages/match/[id]';

describe('normalizeRouteMatchId', () => {
  it.each([
    ['ignores unresolved dynamic placeholder', '[id]', '/match/[id]', ''],
    ['prefers resolved query public id', '01JXND5722FMRTG833TZ2JF621', '/match/ignored', '01976ad2-9c42-7d31-a820-63d7c5279841'],
    ['falls back to resolved path public id', undefined, '/match/01JXND5722FMRTG833TZ2JF621?x=1', '01976ad2-9c42-7d31-a820-63d7c5279841'],
    ['decodes compact UUID routes', '01JXND5722FMRTG833TZ2JF621', '/match/ignored', '01976ad2-9c42-7d31-a820-63d7c5279841']
  ] as const)('%s', (_name, queryID, asPath, expected) => {
    expect(normalizeRouteMatchId(queryID, asPath)).toBe(expected);
  });
});
