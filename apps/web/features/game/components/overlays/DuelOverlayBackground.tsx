import React, { ReactNode } from 'react';

type Props = {
    children?: ReactNode;
    variant?: "duel" | "points";
};

export default function DuelOverlayBackground({ children, variant = "duel" }: Props) {
    const isPoints = variant === "points";
    return (
        <div className="absolute inset-0 z-base overflow-hidden bg-surface-page pointer-events-none">
            {isPoints ? (
                <div className="absolute inset-0 bg-surface-page" />
            ) : (
                <>
                    {/* Left Player Side (Green / Ranked theme) */}
                    <div
                        className="absolute inset-0 bg-status-success/25"
                        style={{ clipPath: 'polygon(0 0, 68% 0, 52% 50%, 45% 50%, 31% 100%, 0 100%)' }}
                    />

                    {/* Right Player Side (Blue / Casual theme) */}
                    <div
                        className="absolute inset-0 bg-brand-blue/20"
                        style={{ clipPath: 'polygon(68% 0, 100% 0, 100% 100%, 31% 100%, 45% 50%, 52% 50%)' }}
                    />
                </>
            )}

            {/* Content wrapper */}
            <div className="absolute inset-0 z-content">{children}</div>
        </div>
    );
}
