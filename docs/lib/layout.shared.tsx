import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import PlatriumLogo from '@/components/platrium_logo'

export function baseOptions(): BaseLayoutProps {
    return {
        nav: {
            title: (
                <div className="flex items-center gap-2">
                    <PlatriumLogo className="size-7" />
                    <span className="font-semibold">Platrium Docs</span>
                </div>
            ),
        },
    };
}
