import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

export function CardListSkeleton({ count = 3 }: { count?: number }) {
  return (
    <div className="space-y-2" aria-busy="true" aria-label="加载中">
      {Array.from({ length: count }, (_, i) => (
        <Card key={i} className="shadow-none">
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between gap-4">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-3 w-24" />
            </div>
          </CardHeader>
          <CardContent className="space-y-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-4/5" />
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

export function DestinationListSkeleton({ count = 2 }: { count?: number }) {
  return (
    <div className="space-y-3" aria-busy="true" aria-label="加载中">
      {Array.from({ length: count }, (_, i) => (
        <Card key={i} className="shadow-none">
          <CardContent className="flex items-center justify-between py-4">
            <div className="flex items-center gap-3">
              <Skeleton className="h-9 w-9 rounded-sm" />
              <div className="space-y-2">
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-3 w-40" />
              </div>
            </div>
            <Skeleton className="h-6 w-16 rounded-full" />
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
