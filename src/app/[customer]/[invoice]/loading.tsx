export default function Loading() {
  return (
    <div className="container mx-auto py-8 px-4 max-w-5xl">
      <div className="mb-6 flex items-center justify-between">
        <div className="h-9 w-32 animate-pulse rounded-md bg-muted"></div>
        <div className="h-9 w-40 animate-pulse rounded-md bg-muted"></div>
      </div>
      <div className="animate-pulse rounded-md border bg-background p-8 space-y-6">
        <div className="h-6 bg-muted rounded w-2/3"></div>
        <div className="h-4 bg-muted rounded w-1/3"></div>
        <div className="space-y-2 pt-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-8 bg-muted rounded"></div>
          ))}
        </div>
      </div>
    </div>
  )
}
