import { describe, expect, it } from 'vitest';
import { buildGoogleMapsLocationUrl, createActualLocationIcon, createTeamPingIcon } from './GuessMap';

describe('team ping markers', () => {
  it('renders a purple pulse with an exclamation mark', () => {
    const icon = createTeamPingIcon();
    const html = String(icon.options.html);

    expect(html).toContain('team-ping');
    expect(html).toContain('team-ping-ring');
    expect(html).toContain('team-ping-core');
    expect(html).toContain('!');
    expect(icon.options.iconSize).toEqual([96, 96]);
    expect(icon.options.iconAnchor).toEqual([48, 48]);
  });
});


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
