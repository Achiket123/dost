// Vertex shader
float4 main(float4 position : POSITION) : SV_POSITION
{
    return position;
}

// Pixel shader
float4 main(float4 position : SV_POSITION) : SV_TARGET
{
    return float4(1.0f, 1.0f, 1.0f, 1.0f);
}