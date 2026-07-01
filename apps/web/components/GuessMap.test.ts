import { describe, expect, it } from 'vitest';
import { buildGoogleMapsLocationUrl, createActualLocationIcon } from './GuessMap';

describe('actual location links', () => {
  it('renders a native Google Maps link in the marker icon', () => {
    const icon = createActualLocationIcon(40.7128, -74.006, 3);
    const html = String(icon.options.html);

    expect(html).toContain(`href="${buildGoogleMapsLocationUrl(40.7128, -74.006).replace(/&/g, '&amp;')}"`);
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
    expect(html).toContain('aria-label="Open round 3 actual location in Google Maps"');
  });
});
