import { useQuery } from '@tanstack/react-query'
import { aiApi } from '@/features/ai/api/ai'

export function useAIAvailability() {
    const query = useQuery({
        queryKey: ['ai', 'config'],
        queryFn: aiApi.config,
        retry: false,
        staleTime: 60_000,
    })

    const enabledProviders = query.data?.providers?.filter((provider) => provider.enabled) ?? []
    const isAIAvailable = !!query.data?.enabled && enabledProviders.length > 0

    return {
        ...query,
        enabledProviders,
        isAIAvailable,
    }
}
