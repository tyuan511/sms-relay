import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90",
        secondary: "bg-card text-foreground border border-border hover:bg-muted",
        outline: "border border-border bg-card text-foreground hover:bg-muted",
        ghost: "text-body hover:bg-muted hover:text-foreground",
        destructive: "bg-destructive text-white hover:bg-destructive/90",
        link: "text-accent underline-offset-4 hover:underline p-0 h-auto",
      },
      size: {
        default: "h-9 rounded-sm px-3",
        sm: "h-7 rounded-sm px-2.5 text-xs",
        lg: "h-10 rounded-sm px-4 text-sm",
        pill: "h-10 rounded-full px-5",
        icon: "h-9 w-9 rounded-sm",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean
}

export function Button({ className, variant, size, asChild = false, ...props }: ButtonProps) {
  const Comp = asChild ? Slot : "button"
  return <Comp className={cn(buttonVariants({ variant, size, className }))} {...props} />
}
