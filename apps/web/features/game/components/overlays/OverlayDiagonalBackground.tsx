import React from 'react';

export default function OverlayDiagonalBackground() {
  return (
    <div className="absolute inset-0 z-base flex pointer-events-none">
      <div className="absolute inset-0 bg-brand-orange/80" />
      <div 
        className="absolute inset-0 bg-brand-blue/70 shadow-elev-3"
        style={{ 
          clipPath: 'polygon(55% 0, 100% 0, 100% 100%, 35% 100%)',
        }} 
      />
      {/* To get a shadow-elev-1 on the clipped edge, we can add an SVG or another div */}
      <div 
        className="absolute inset-0 bg-scrim w-full"
        style={{ 
            clipPath: 'polygon(55% 0, 56% 0, 36% 100%, 35% 100%)',
            filter: 'blur(8px)'
        }} 
      />
    </div>
  );
}
