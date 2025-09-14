// Structure for color (RGBA)
typedef struct {
    float r, g, b, a;
} Color;

// Simple fragment shader that outputs a constant color
Color fragmentShader() {
    Color color = { 1.0f, 0.0f, 0.0f, 1.0f }; // Red color
    return color;
}
